// Package controller provides the Service reconciler for the koptan operator.
// It discovers source languages, generates Dockerfiles, and manages the
// CI/CD pipeline lifecycle. When a Ready Service detects a new git commit,
// it points its CI at the new commit (discover → build → deploy).
package controller

import (
	"context"
	"fmt"
	"time"

	koptanv1 "github.com/felukka/koptan/api/v1"
	"github.com/felukka/koptan/internal/service"
	"github.com/felukka/koptan/internal/utils"
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
// +kubebuilder:rbac:groups=koptan.felukka.org,resources=ciplugins,verbs=get;list;watch
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

	pluginRefs, err := r.resolvePlugins(ctx, &svc)
	if err != nil {
		r.markFailed(&svc, "PluginUnavailable", err.Error())
		return ctrl.Result{RequeueAfter: failedRetryInterval}, r.patchStatus(ctx, orig, &svc)
	}

	var rev *utils.Revision
	if specChanged || svc.Status.Phase != koptanv1.ServicePhaseReady {
		rev, err = utils.ResolveRevision(ctx, svc.Spec.Source.Repo, svc.Spec.Source.Revision, token)
		if err != nil {
			r.markFailed(&svc, "RevisionUnresolved", fmt.Sprintf("resolve revision: %v", err))
			return ctrl.Result{RequeueAfter: failedRetryInterval}, r.patchStatus(ctx, orig, &svc)
		}
	} else if rev, err = r.pollForCommit(ctx, &svc, token); err != nil {
		log.Error(err, "failed to check for new commits, will retry")
		svc.Status.Message = fmt.Sprintf("Could not check for new commits: %v", err)
		return ctrl.Result{RequeueAfter: pushPollInterval}, r.patchStatus(ctx, orig, &svc)
	}

	if rev != nil {
		if res, err := r.discoverAndBuild(ctx, &svc, rev, token, pluginRefs); err != nil || !res.IsZero() {
			if perr := r.patchStatus(ctx, orig, &svc); perr != nil {
				return ctrl.Result{}, perr
			}
			return res, err
		}
	}

	if err := r.syncPlugins(ctx, &svc, pluginRefs); err != nil {
		return ctrl.Result{}, err
	}
	if err := r.syncCD(ctx, &svc); err != nil {
		return ctrl.Result{}, err
	}
	if err := r.patchStatus(ctx, orig, &svc); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: pushPollInterval}, nil
}

// pollForCommit returns the new revision when the tracked branch or tag
// moved since the last build, or nil when it did not.
func (r *ServiceReconciler) pollForCommit(ctx context.Context, svc *koptanv1.Service, token string) (*utils.Revision, error) {
	changed, rev, err := utils.HasRevisionChanged(ctx, svc.Spec.Source.Repo,
		svc.Spec.Source.Revision, svc.Status.LatestRevision, token)
	if err != nil || !changed {
		return nil, err
	}
	logf.FromContext(ctx).Info("new commit detected, re-triggering pipeline",
		"previousRevision", svc.Status.LatestRevision, "newRevision", rev.SHA)
	now := metav1.Now()
	svc.Status.LastPushDetected = &now
	meta.SetStatusCondition(&svc.Status.Conditions, metav1.Condition{
		Type:    "CommitDetected",
		Status:  metav1.ConditionTrue,
		Reason:  "NewCommit",
		Message: fmt.Sprintf("New commit %s detected on %s", short(rev.SHA), svc.Spec.Source.Repo),
	})
	return rev, nil
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
		Watches(&koptanv1.CIPlugin{}, handler.EnqueueRequestsFromMapFunc(r.servicesForPlugin)).
		Named("service").
		Complete(r)
}
