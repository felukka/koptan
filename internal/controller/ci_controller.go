package controller

import (
	"context"
	"fmt"
	"time"

	koptanv1 "github.com/felukka/koptan/api/v1"
	"github.com/felukka/koptan/internal/utils"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	ciFinalizer = "felukka.org/ci-cleanup"
	// buildPollInterval is how often a running build Pod is checked; Pod
	// events also trigger a reconcile.
	buildPollInterval = 15 * time.Second
)

// CIReconciler builds the image for a Service in a build Pod (git clone +
// buildah), then creates or updates the CD that deploys it.
type CIReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	// Secrets decides which Secrets a build Pod may use.
	Secrets SecretPolicy
	// APIReader reads around the informer cache, to tell a build Pod that
	// is really gone from one the cache has not seen yet. Defaults to the
	// client.
	APIReader client.Reader
}

// NewCIReconciler returns a CIReconciler.
func NewCIReconciler(c client.Client, s *runtime.Scheme) *CIReconciler {
	return &CIReconciler{Client: c, Scheme: s, APIReader: c}
}

// +kubebuilder:rbac:groups=koptan.felukka.org,resources=cis,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=koptan.felukka.org,resources=cis/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=koptan.felukka.org,resources=cis/finalizers,verbs=update
// +kubebuilder:rbac:groups=koptan.felukka.org,resources=services,verbs=get;list;watch
// +kubebuilder:rbac:groups=koptan.felukka.org,resources=ciplugins,verbs=get;list;watch
// +kubebuilder:rbac:groups=koptan.felukka.org,resources=cds,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;delete;deletecollection
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch

// Reconcile builds spec.revision once per revision and spec generation.
// A failed build is not retried until the revision or the spec changes.
func (r *CIReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	res, err := r.reconcile(ctx, req)
	if apierrors.IsConflict(err) {
		// The status write was based on a stale copy (see patchStatus).
		return ctrl.Result{Requeue: true}, nil
	}
	return res, err
}

func (r *CIReconciler) reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var ci koptanv1.CI
	if err := r.Get(ctx, req.NamespacedName, &ci); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !ci.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(&ci, ciFinalizer) {
			controllerutil.RemoveFinalizer(&ci, ciFinalizer)
			return ctrl.Result{}, r.Update(ctx, &ci)
		}
		return ctrl.Result{}, nil
	}
	if !controllerutil.ContainsFinalizer(&ci, ciFinalizer) {
		controllerutil.AddFinalizer(&ci, ciFinalizer)
		if err := r.Update(ctx, &ci); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	orig := ci.DeepCopy()
	svc, err := r.resolveService(ctx, &ci)
	if err != nil {
		setCIPhase(&ci, koptanv1.CIPhaseIdle, fmt.Sprintf("Waiting for Service %q: %v", ci.Spec.Service.Name, err))
		return ctrl.Result{RequeueAfter: time.Minute}, r.patchStatus(ctx, orig, &ci)
	}
	sha := ci.Spec.Revision
	if sha == "" || ci.Spec.DockerfileConfigMap == "" {
		setCIPhase(&ci, koptanv1.CIPhaseIdle, fmt.Sprintf("Waiting for Service %q to finish discovery", svc.Name))
		return ctrl.Result{}, r.patchStatus(ctx, orig, &ci)
	}
	if err := validateBuild(&ci, svc); err != nil {
		r.markCIFailed(&ci, "InvalidSpec", err.Error())
		ci.Status.BuildingRevision = sha
		return ctrl.Result{}, r.patchStatus(ctx, orig, &ci)
	}

	image := imageRef(&ci, sha)
	current := ci.Status.BuildingRevision == sha && ci.Status.ObservedGeneration == ci.Generation

	// --- A build for this revision and spec is known: follow it. ---
	if current && ci.Status.BuildPod != "" && ci.Status.Phase == koptanv1.CIPhaseBuilding {
		return r.followBuild(ctx, orig, &ci, svc, sha, image)
	}

	// --- Up to date, or failed for this exact revision and spec: nothing to build. ---
	if current && (ci.Status.Phase == koptanv1.CIPhaseSucceeded || ci.Status.Phase == koptanv1.CIPhaseFailed) {
		if ci.Status.Phase == koptanv1.CIPhaseSucceeded {
			return ctrl.Result{}, r.ensureCD(ctx, &ci, svc)
		}
		return ctrl.Result{}, nil
	}

	// --- Start a new build. ---
	setCIPhase(&ci, koptanv1.CIPhaseResolving, fmt.Sprintf("Preparing build of %s", short(sha)))
	dockerCfg, err := r.registrySecret(ctx, &ci)
	if err != nil {
		r.markCIFailed(&ci, "RegistryCredentials", err.Error())
		ci.Status.BuildingRevision = sha
		ci.Status.ObservedGeneration = ci.Generation
		return ctrl.Result{}, r.patchStatus(ctx, orig, &ci)
	}
	steps, err := r.pluginSteps(ctx, &ci, svc, sha)
	if err != nil {
		r.markCIFailed(&ci, "PluginUnavailable", err.Error())
		ci.Status.BuildingRevision = sha
		ci.Status.ObservedGeneration = ci.Generation
		return ctrl.Result{}, r.patchStatus(ctx, orig, &ci)
	}
	if err := r.deleteBuildPods(ctx, &ci); err != nil {
		return ctrl.Result{}, err
	}
	pod := buildPod(&ci, svc, sha, image, dockerCfg, steps)
	if err := r.Secrets.checkPod(ctx, r, pod); err != nil {
		r.markCIFailed(&ci, "SecretNotAllowed", err.Error())
		ci.Status.BuildingRevision = sha
		ci.Status.ObservedGeneration = ci.Generation
		return ctrl.Result{}, r.patchStatus(ctx, orig, &ci)
	}
	if err := controllerutil.SetControllerReference(&ci, pod, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}
	if err := r.Create(ctx, pod); err != nil {
		// Show why (e.g. "exceeded quota") instead of only logging it, and
		// leave the revision and generation unobserved so the build is
		// tried again once whatever blocked it is fixed.
		gen := ci.Status.ObservedGeneration
		r.markCIFailed(&ci, "PodCreateFailed", fmt.Sprintf("create build pod: %v", err))
		ci.Status.ObservedGeneration = gen
		if perr := r.patchStatus(ctx, orig, &ci); perr != nil {
			return ctrl.Result{}, perr
		}
		return ctrl.Result{RequeueAfter: failedRetryInterval}, nil
	}
	log.Info("starting build", "service", svc.Name, "image", image, "pod", pod.Name)
	ci.Status.BuildPod = pod.Name
	ci.Status.PluginResults = pendingPluginResults(&ci)
	ci.Status.BuildingRevision = sha
	ci.Status.ObservedGeneration = ci.Generation
	setCIPhase(&ci, koptanv1.CIPhaseBuilding, fmt.Sprintf("Building %s in pod %s", image, pod.Name))
	return ctrl.Result{RequeueAfter: buildPollInterval}, r.patchStatus(ctx, orig, &ci)
}

// followBuild tracks the running build Pod and records its outcome; on
// success it creates or updates the CD.
func (r *CIReconciler) followBuild(ctx context.Context, orig, ci *koptanv1.CI, svc *koptanv1.Service,
	sha, image string) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	pod, lost, err := r.buildPodOf(ctx, ci)
	if err != nil {
		return ctrl.Result{}, err
	}
	if lost {
		r.markCIFailed(ci, "BuildPodLost", fmt.Sprintf("build pod %s disappeared", ci.Status.BuildPod))
		return ctrl.Result{}, r.patchStatus(ctx, orig, ci)
	}
	if pod == nil { // this reconcile saw a stale CI; a fresh one follows
		return ctrl.Result{Requeue: true}, nil
	}
	phase, msg, done := podResult(pod)
	ci.Status.PluginResults = pluginResults(ci, pod)
	if !done {
		setCIPhase(ci, koptanv1.CIPhaseBuilding, msg)
		return ctrl.Result{RequeueAfter: buildPollInterval}, r.patchStatus(ctx, orig, ci)
	}
	if phase == koptanv1.CIPhaseFailed {
		log.Info("build failed", "pod", pod.Name, "reason", msg)
		r.markCIFailed(ci, "BuildFailed", msg)
		return ctrl.Result{}, r.patchStatus(ctx, orig, ci)
	}
	log.Info("build succeeded", "image", image)
	now := metav1.Now()
	ci.Status.Phase = koptanv1.CIPhaseSucceeded
	ci.Status.Image = image
	ci.Status.Revision = sha
	ci.Status.BuildTime = &now
	ci.Status.BuildCount++
	ci.Status.Message = "build succeeded"
	meta.SetStatusCondition(&ci.Status.Conditions, metav1.Condition{
		Type: "BuildSucceeded", Status: metav1.ConditionTrue, Reason: "BuildCompleted",
		Message: fmt.Sprintf("Image %s", image),
	})
	meta.SetStatusCondition(&ci.Status.Conditions, metav1.Condition{
		Type: "Ready", Status: metav1.ConditionTrue, Reason: "Succeeded", Message: "build succeeded",
	})
	if err := r.patchStatus(ctx, orig, ci); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, r.ensureCD(ctx, ci, svc)
}

func validateBuild(ci *koptanv1.CI, svc *koptanv1.Service) error {
	if err := utils.ValidateGitURL(svc.Spec.Source.Repo); err != nil {
		return err
	}
	if !utils.IsSHA(ci.Spec.Revision) {
		return fmt.Errorf("spec.revision must be a full commit SHA, got %q", ci.Spec.Revision)
	}
	if ci.Spec.Registry.Registry == "" || ci.Spec.Registry.Repo == "" {
		return fmt.Errorf("spec.image.registry and spec.image.repo are required")
	}
	return utils.ValidateRelPath("spec.contextDir", ci.Spec.ContextDir)
}

// registrySecret returns the dockerconfigjson Secret used to push: the
// credentialsSecret, or one generated from the deprecated loginSecret.
func (r *CIReconciler) registrySecret(ctx context.Context, ci *koptanv1.CI) (string, error) {
	if name := ci.Spec.Registry.CredentialsSecret; name != "" {
		s, err := r.Secrets.get(ctx, r, ci.Namespace, name)
		if err != nil {
			return "", fmt.Errorf("registry credentials: %w", err)
		}
		if _, ok := s.Data[corev1.DockerConfigJsonKey]; !ok {
			return "", fmt.Errorf("secret %q has no %s key (type kubernetes.io/dockerconfigjson expected)",
				name, corev1.DockerConfigJsonKey)
		}
		return name, nil
	}
	creds := ci.Spec.Registry.Creds //nolint:staticcheck // deprecated Creds still supported for backward compatibility
	if creds == nil {
		return "", nil
	}
	cfg, err := buildDockerConfigJSON(ci.Spec.Registry.Registry, creds.Username, string(creds.Password))
	if err != nil {
		return "", err
	}
	s := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: ci.Name + "-registry", Namespace: ci.Namespace}}
	_, err = controllerutil.CreateOrUpdate(ctx, r.Client, s, func() error {
		s.Labels = map[string]string{labelCredentials: "true"}
		s.Type = corev1.SecretTypeDockerConfigJson
		s.Data = map[string][]byte{corev1.DockerConfigJsonKey: cfg}
		return controllerutil.SetControllerReference(ci, s, r.Scheme)
	})
	return s.Name, err
}

// deleteBuildPods removes earlier build Pods of this CI.
func (r *CIReconciler) deleteBuildPods(ctx context.Context, ci *koptanv1.CI) error {
	return client.IgnoreNotFound(r.DeleteAllOf(ctx, &corev1.Pod{}, client.InNamespace(ci.Namespace),
		client.MatchingLabels{labelCI: ci.Name}, client.PropagationPolicy(metav1.DeletePropagationBackground)))
}

func (r *CIReconciler) resolveService(ctx context.Context, ci *koptanv1.CI) (*koptanv1.Service, error) {
	svc := &koptanv1.Service{}
	key := types.NamespacedName{Name: ci.Spec.Service.Name, Namespace: ci.Namespace}
	if err := r.Get(ctx, key, svc); err != nil {
		return nil, err
	}
	return svc, nil
}

// ensureCD creates or updates <service>-cd for the built image.
func (r *CIReconciler) ensureCD(ctx context.Context, ci *koptanv1.CI, svc *koptanv1.Service) error {
	cd := &koptanv1.CD{ObjectMeta: metav1.ObjectMeta{Name: svc.Name + "-cd", Namespace: ci.Namespace}}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, cd, func() error {
		if cd.Labels == nil {
			cd.Labels = map[string]string{}
		}
		cd.Labels[labelService] = svc.Name
		cd.Labels[labelCI] = ci.Name
		cd.Spec.CI.Name = ci.Name
		cd.Spec.ImagePullSecret = ci.Spec.Registry.CredentialsSecret
		if cd.Spec.ImagePullSecret == "" && ci.Spec.Registry.Creds != nil { //nolint:staticcheck // deprecated Creds still supported for backward compatibility
			cd.Spec.ImagePullSecret = ci.Name + "-registry"
		}
		applyServiceToCD(svc, cd)
		if cd.Spec.Resources == nil {
			cd.Spec.Resources = &koptanv1.Resources{
				CPURequest:    ptrQ("100m"),
				CPULimit:      ptrQ("500m"),
				MemoryRequest: ptrQ("128Mi"),
				MemoryLimit:   ptrQ("256Mi"),
			}
		}
		if metav1.GetControllerOf(cd) == nil {
			return controllerutil.SetControllerReference(ci, cd, r.Scheme)
		}
		return nil
	})
	return err
}

// buildPodOf returns the Pod status.buildPod names. The cache can lag
// behind: right after a new build starts, a reconcile may still see the
// previous Pod name, already deleted, or a new Pod not cached yet. So a
// cache miss is checked against the API server: lost is true only when
// the Pod is really gone and the CI still names it. A nil Pod without
// lost means this reconcile's CI is stale.
func (r *CIReconciler) buildPodOf(ctx context.Context, ci *koptanv1.CI) (*corev1.Pod, bool, error) {
	key := types.NamespacedName{Name: ci.Status.BuildPod, Namespace: ci.Namespace}
	var pod corev1.Pod
	err := r.Get(ctx, key, &pod)
	if err == nil || !apierrors.IsNotFound(err) {
		return &pod, false, err
	}
	reader := r.APIReader
	if reader == nil {
		reader = r.Client
	}
	if err := reader.Get(ctx, key, &pod); err == nil {
		return &pod, false, nil
	} else if !apierrors.IsNotFound(err) {
		return nil, false, err
	}
	var fresh koptanv1.CI
	if err := reader.Get(ctx, client.ObjectKeyFromObject(ci), &fresh); err != nil {
		return nil, false, client.IgnoreNotFound(err)
	}
	return nil, fresh.Status.BuildPod == ci.Status.BuildPod, nil
}

// patchStatus writes status changes as a merge patch with an optimistic
// lock: a reconcile that started from a stale copy conflicts and is
// requeued instead of writing over newer status (such as a newer build Pod).
func (r *CIReconciler) patchStatus(ctx context.Context, orig, ci *koptanv1.CI) error {
	return r.Status().Patch(ctx, ci, client.MergeFromWithOptions(orig, client.MergeFromWithOptimisticLock{}))
}

func setCIPhase(ci *koptanv1.CI, phase koptanv1.CIPhase, message string) {
	ci.Status.Phase = phase
	ci.Status.Message = message
	meta.SetStatusCondition(&ci.Status.Conditions, metav1.Condition{
		Type:    "Ready",
		Status:  metav1.ConditionUnknown,
		Reason:  string(phase),
		Message: message,
	})
}

func (r *CIReconciler) markCIFailed(ci *koptanv1.CI, reason, msg string) {
	ci.Status.Phase = koptanv1.CIPhaseFailed
	ci.Status.Message = msg
	ci.Status.ObservedGeneration = ci.Generation
	meta.SetStatusCondition(&ci.Status.Conditions, metav1.Condition{
		Type: "BuildSucceeded", Status: metav1.ConditionFalse, Reason: reason, Message: msg,
	})
	meta.SetStatusCondition(&ci.Status.Conditions, metav1.Condition{
		Type: "Ready", Status: metav1.ConditionFalse, Reason: reason, Message: msg,
	})
}

func ptrQ(s string) *resource.Quantity {
	q := resource.MustParse(s)
	return &q
}

// SetupWithManager sets up the controller with the Manager.
func (r *CIReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&koptanv1.CI{}).
		Owns(&corev1.Pod{}).
		Owns(&koptanv1.CD{}).
		Named("ci").
		Complete(r)
}
