// Package controller provides the Service reconciler for the koptan operator.
// It discovers source languages, generates Dockerfiles, and manages the
// CI/CD pipeline lifecycle. When a Ready Service detects a new git commit,
// it points its CI at the new commit (discover → build → deploy).
package controller

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	koptanv1 "github.com/felukka/koptan/api/v1"
	"github.com/felukka/koptan/internal/service"
	"github.com/felukka/koptan/internal/utils"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	// serviceFinalizer is the finalizer applied to Service resources.
	serviceFinalizer = "felukka.org/service-cleanup"
	// pushPollInterval is how often to check for new commits on Ready services.
	pushPollInterval = 1 * time.Minute
	// failedRetryInterval is how long a Failed Service waits before retrying
	// a transient failure (unreachable git, missing secret).
	failedRetryInterval = 1 * time.Minute

	labelService  = "koptan.felukka.org/service"
	labelLanguage = "koptan.felukka.org/language"
	dockerfileKey = "Dockerfile"
	defaultPort   = 8080
)

// ServiceReconciler reconciles a Service object by discovering the source
// language, storing the Dockerfile, and creating a CI CRD to trigger the
// build pipeline. On Ready services it polls for new git commits.
type ServiceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	// DefaultRegistry is used when a Service sets no spec.image.registry.
	DefaultRegistry string
	// Engine discovers languages; nil means service.NewEngine().
	Engine *service.Engine
}

// +kubebuilder:rbac:groups=koptan.felukka.org,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=koptan.felukka.org,resources=services/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=koptan.felukka.org,resources=services/finalizers,verbs=update
// +kubebuilder:rbac:groups=koptan.felukka.org,resources=cis,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=koptan.felukka.org,resources=cds,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete

// Reconcile implements the Service reconciliation loop.
// Lifecycle: Pending -> Discovering -> Ready (CI created). On Ready it polls
// git and moves the CI to new commits; spec changes trigger a new discovery.
func (r *ServiceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var svc koptanv1.Service
	if err := r.Get(ctx, req.NamespacedName, &svc); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// --- Deletion handling ---
	if !svc.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(&svc, serviceFinalizer) {
			controllerutil.RemoveFinalizer(&svc, serviceFinalizer)
			return ctrl.Result{}, r.Update(ctx, &svc)
		}
		return ctrl.Result{}, nil
	}

	// --- Add finalizer ---
	if !controllerutil.ContainsFinalizer(&svc, serviceFinalizer) {
		controllerutil.AddFinalizer(&svc, serviceFinalizer)
		if err := r.Update(ctx, &svc); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	orig := svc.DeepCopy()
	specChanged := svc.Status.ObservedGeneration != svc.Generation

	// Invalid input never fixes itself: fail once and wait for a spec change.
	if err := validateSource(&svc); err != nil {
		if svc.Status.Phase != koptanv1.ServicePhaseFailed || specChanged {
			r.markFailed(&svc, "InvalidSource", err.Error())
			return ctrl.Result{}, r.patchStatus(ctx, orig, &svc)
		}
		return ctrl.Result{}, nil
	}

	// A failed Service retries transient errors after a pause, not in a loop.
	if svc.Status.Phase == koptanv1.ServicePhaseFailed && !specChanged {
		if wait := retryWait(&svc); wait > 0 {
			return ctrl.Result{RequeueAfter: wait}, nil
		}
	}

	token, err := r.gitToken(ctx, &svc)
	if err != nil {
		r.markFailed(&svc, "SecretUnavailable", err.Error())
		return ctrl.Result{RequeueAfter: failedRetryInterval}, r.patchStatus(ctx, orig, &svc)
	}

	needsDiscovery := specChanged || svc.Status.Phase != koptanv1.ServicePhaseReady
	var rev *utils.Revision
	if needsDiscovery {
		rev, err = utils.ResolveRevision(ctx, svc.Spec.Source.Repo, svc.Spec.Source.Revision, token)
		if err != nil {
			r.markFailed(&svc, "RevisionUnresolved", fmt.Sprintf("resolve revision: %v", err))
			return ctrl.Result{RequeueAfter: failedRetryInterval}, r.patchStatus(ctx, orig, &svc)
		}
	} else {
		// --- Polling phase: check for new git commits ---
		changed, newRev, err := utils.HasRevisionChanged(ctx, svc.Spec.Source.Repo,
			svc.Spec.Source.Revision, svc.Status.LatestRevision, token)
		if err != nil {
			log.Error(err, "failed to check for new commits, will retry")
			svc.Status.Message = fmt.Sprintf("Could not check for new commits: %v", err)
			return ctrl.Result{RequeueAfter: pushPollInterval}, r.patchStatus(ctx, orig, &svc)
		}
		if changed {
			log.Info("new commit detected, re-triggering pipeline",
				"previousRevision", svc.Status.LatestRevision, "newRevision", newRev.SHA)
			now := metav1.Now()
			svc.Status.LastPushDetected = &now
			meta.SetStatusCondition(&svc.Status.Conditions, metav1.Condition{
				Type:    "CommitDetected",
				Status:  metav1.ConditionTrue,
				Reason:  "NewCommit",
				Message: fmt.Sprintf("New commit %s detected on %s", short(newRev.SHA), svc.Spec.Source.Repo),
			})
			rev = newRev
		}
	}

	if rev != nil {
		if res, err := r.discoverAndBuild(ctx, &svc, rev, token); err != nil || !res.IsZero() {
			if perr := r.patchStatus(ctx, orig, &svc); perr != nil {
				return ctrl.Result{}, perr
			}
			return res, err
		}
	}

	if err := r.syncCD(ctx, &svc); err != nil {
		return ctrl.Result{}, err
	}
	if err := r.patchStatus(ctx, orig, &svc); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: pushPollInterval}, nil
}

// discoverAndBuild runs discovery at rev, stores the Dockerfile and points
// the CI at rev. A non-zero result or error means the Service is not Ready.
func (r *ServiceReconciler) discoverAndBuild(ctx context.Context, svc *koptanv1.Service,
	rev *utils.Revision, token string) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	setPhase(svc, koptanv1.ServicePhaseDiscovering,
		fmt.Sprintf("Discovering %s at %s", svc.Spec.Source.Repo, short(rev.SHA)))

	engine := r.Engine
	if engine == nil {
		engine = service.NewEngine()
	}
	result, err := engine.Discover(ctx, svc.Spec.Source.Repo, rev, token)
	if err != nil {
		r.markFailed(svc, "DiscoveryFailed", fmt.Sprintf("discover: %v", err))
		return ctrl.Result{RequeueAfter: failedRetryInterval}, nil
	}
	log.Info("discovery complete", "language", result.Language, "hasDockerfile", result.HasDockerfile)

	cmName, err := r.ensureDockerfile(ctx, svc, result.Dockerfile)
	if err != nil {
		r.markFailed(svc, "DockerfileFailed", fmt.Sprintf("store Dockerfile: %v", err))
		return ctrl.Result{}, err
	}

	ciName, err := r.ensureCI(ctx, svc, result.Language, rev.SHA, cmName)
	if err != nil {
		if apierrors.IsConflict(err) || apierrors.IsAlreadyExists(err) {
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}
		r.markFailed(svc, "CIInitFailed", fmt.Sprintf("create CI: %v", err))
		return ctrl.Result{}, err
	}

	svc.Status.ServiceType = result.Language
	svc.Status.LatestRevision = rev.SHA
	svc.Status.DockerfileConfigMap = cmName
	svc.Status.CIRef = ciName
	svc.Status.ObservedGeneration = svc.Generation
	svc.Status.Error = ""
	meta.SetStatusCondition(&svc.Status.Conditions, metav1.Condition{
		Type:   "DockerfileGenerated",
		Status: metav1.ConditionTrue,
		Reason: "Generated",
		Message: fmt.Sprintf("Dockerfile %s for %s",
			map[bool]string{true: "detected", false: "generated"}[result.HasDockerfile],
			result.Language),
	})
	setPhase(svc, koptanv1.ServicePhaseReady,
		fmt.Sprintf("CI %s building %s", ciName, short(rev.SHA)))
	meta.SetStatusCondition(&svc.Status.Conditions, metav1.Condition{
		Type: "Ready", Status: metav1.ConditionTrue, Reason: "Discovered",
		Message: svc.Status.Message,
	})
	return ctrl.Result{}, nil
}

// ensureDockerfile stores the Dockerfile in ConfigMap
// <svc>-dockerfile-<hash>. The hash in the name changes the CI spec when the
// Dockerfile changes, so CI rebuilds; older ConfigMaps are removed.
func (r *ServiceReconciler) ensureDockerfile(ctx context.Context, svc *koptanv1.Service, dockerfile []byte) (string, error) {
	sum := sha256.Sum256(dockerfile)
	cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
		Name: fmt.Sprintf("%s-dockerfile-%x", svc.Name, sum[:4]), Namespace: svc.Namespace,
	}}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, cm, func() error {
		if cm.Labels == nil {
			cm.Labels = map[string]string{}
		}
		cm.Labels[labelService] = svc.Name
		cm.Data = map[string]string{dockerfileKey: string(dockerfile)}
		return controllerutil.SetControllerReference(svc, cm, r.Scheme)
	}); err != nil {
		return "", err
	}
	var old corev1.ConfigMapList
	if err := r.List(ctx, &old, client.InNamespace(svc.Namespace),
		client.MatchingLabels{labelService: svc.Name}); err != nil {
		return "", err
	}
	for i := range old.Items {
		if old.Items[i].Name != cm.Name && metav1.IsControlledBy(&old.Items[i], svc) {
			if err := r.Delete(ctx, &old.Items[i]); client.IgnoreNotFound(err) != nil {
				return "", err
			}
		}
	}
	return cm.Name, nil
}

// ensureCI creates or updates the CI that builds this Service. An existing
// CI for the Service (e.g. one applied by hand) is reused.
func (r *ServiceReconciler) ensureCI(ctx context.Context, svc *koptanv1.Service,
	language, sha, dockerfileCM string) (string, error) {
	name := svc.Name + "-ci"
	var ciList koptanv1.CIList
	if err := r.List(ctx, &ciList, client.InNamespace(svc.Namespace)); err != nil {
		return "", err
	}
	for _, c := range ciList.Items {
		if c.Spec.Service.Name == svc.Name {
			if !c.DeletionTimestamp.IsZero() {
				return "", apierrors.NewConflict(koptanv1.GroupVersion.WithResource("cis").GroupResource(),
					c.Name, fmt.Errorf("CI is being deleted"))
			}
			name = c.Name
			break
		}
	}

	ci := &koptanv1.CI{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: svc.Namespace}}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, ci, func() error {
		if ci.Labels == nil {
			ci.Labels = map[string]string{}
		}
		ci.Labels[labelService] = svc.Name
		ci.Labels[labelLanguage] = language
		ci.Spec.Service.Name = svc.Name
		ci.Spec.Revision = sha
		ci.Spec.DockerfileConfigMap = dockerfileCM

		img := svc.Spec.Image
		switch {
		case img != nil:
			ci.Spec.Registry.Registry = img.Registry
			ci.Spec.Registry.Repo = img.Repo
			ci.Spec.Registry.CredentialsSecret = img.CredentialsSecret
		case ci.Spec.Registry.Registry != "":
			// Keep a registry set on the CI directly.
		}
		if ci.Spec.Registry.Registry == "" {
			ci.Spec.Registry.Registry = r.DefaultRegistry
		}
		if ci.Spec.Registry.Registry == "" {
			ci.Spec.Registry.Registry = "docker.io"
		}
		if ci.Spec.Registry.Repo == "" {
			ci.Spec.Registry.Repo = svc.Name
		}
		if metav1.GetControllerOf(ci) == nil {
			return controllerutil.SetControllerReference(svc, ci, r.Scheme)
		}
		return nil
	})
	return name, err
}

// syncCD copies replicas, env and port from the Service to its CD and
// records the CD's name.
func (r *ServiceReconciler) syncCD(ctx context.Context, svc *koptanv1.Service) error {
	var cds koptanv1.CDList
	if err := r.List(ctx, &cds, client.InNamespace(svc.Namespace),
		client.MatchingLabels{labelService: svc.Name}); err != nil {
		return err
	}
	svc.Status.CDRef = ""
	for i := range cds.Items {
		cd := &cds.Items[i]
		if !cd.DeletionTimestamp.IsZero() {
			continue
		}
		svc.Status.CDRef = cd.Name
		patch := client.MergeFrom(cd.DeepCopy())
		applyServiceToCD(svc, cd)
		if err := r.Patch(ctx, cd, patch); err != nil {
			return client.IgnoreNotFound(err)
		}
	}
	return nil
}

// applyServiceToCD sets the CD fields that the Service owns.
func applyServiceToCD(svc *koptanv1.Service, cd *koptanv1.CD) {
	cd.Spec.Replicas = 1
	if svc.Spec.Replicas != nil {
		cd.Spec.Replicas = *svc.Spec.Replicas
	}
	cd.Spec.Env = svc.Spec.Env
	cd.Spec.Port = svc.Spec.Port
	if cd.Spec.Port == 0 {
		cd.Spec.Port = defaultPort
	}
}

// gitToken reads the token named by spec.source.secretRef, if any.
func (r *ServiceReconciler) gitToken(ctx context.Context, svc *koptanv1.Service) (string, error) {
	ref := svc.Spec.Source.SecretRef
	if ref == nil {
		return "", nil
	}
	var secret corev1.Secret
	if err := r.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: svc.Namespace}, &secret); err != nil {
		return "", fmt.Errorf("read git token secret %q: %w", ref.Name, err)
	}
	key := ref.Key
	if key == "" {
		key = "token"
	}
	token, ok := secret.Data[key]
	if !ok {
		return "", fmt.Errorf("secret %q has no key %q", ref.Name, key)
	}
	return string(token), nil
}

func validateSource(svc *koptanv1.Service) error {
	if err := utils.ValidateGitURL(svc.Spec.Source.Repo); err != nil {
		return err
	}
	return utils.ValidateRevision(svc.Spec.Source.Revision)
}

// retryWait is how long a Failed Service still waits before retrying.
func retryWait(svc *koptanv1.Service) time.Duration {
	c := meta.FindStatusCondition(svc.Status.Conditions, "Ready")
	if c == nil {
		return 0
	}
	return time.Until(c.LastTransitionTime.Add(failedRetryInterval))
}

// patchStatus writes status changes as a merge patch, so concurrent spec or
// metadata updates do not make the write fail.
func (r *ServiceReconciler) patchStatus(ctx context.Context, orig, svc *koptanv1.Service) error {
	return r.Status().Patch(ctx, svc, client.MergeFrom(orig))
}

// setPhase sets the phase and message on the Service status.
func setPhase(svc *koptanv1.Service, phase koptanv1.ServicePhase, message string) {
	svc.Status.Phase = phase
	svc.Status.Message = message
	if phase != koptanv1.ServicePhaseReady {
		meta.SetStatusCondition(&svc.Status.Conditions, metav1.Condition{
			Type:    "Ready",
			Status:  metav1.ConditionUnknown,
			Reason:  string(phase),
			Message: message,
		})
	}
}

// markFailed marks the Service as Failed with the given reason and message.
func (r *ServiceReconciler) markFailed(svc *koptanv1.Service, reason, msg string) {
	svc.Status.Phase = koptanv1.ServicePhaseFailed
	svc.Status.Error = msg
	svc.Status.Message = msg
	svc.Status.ObservedGeneration = svc.Generation
	meta.RemoveStatusCondition(&svc.Status.Conditions, "Ready")
	meta.SetStatusCondition(&svc.Status.Conditions, metav1.Condition{
		Type:    "Ready",
		Status:  metav1.ConditionFalse,
		Reason:  reason,
		Message: msg,
	})
}

func short(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}

// SetupWithManager sets up the controller with the Manager.
func (r *ServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&koptanv1.Service{}).
		Owns(&koptanv1.CI{}).
		Watches(&koptanv1.CD{}, handler.EnqueueRequestsFromMapFunc(
			func(_ context.Context, o client.Object) []ctrl.Request {
				name := o.GetLabels()[labelService]
				if name == "" {
					return nil
				}
				return []ctrl.Request{{NamespacedName: types.NamespacedName{
					Namespace: o.GetNamespace(), Name: name}}}
			})).
		Named("service").
		Complete(r)
}
