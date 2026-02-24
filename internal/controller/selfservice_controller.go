package controller

import (
	"context"
	"fmt"
	"net/http"
	"time"

	koptanv1 "github.com/felukka/koptan/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const selfServiceFinalizer = "felukka.org/selfservice-cleanup"

// SelfServiceReconciler pairs a git repository with an agent session: it
// creates the repository when asked, runs the koptan-agent Deployment and
// its Service, and creates the koptan Service that builds and deploys
// what the agent pushes. Deleting a SelfService deletes the session and
// the Service (owner references) but never the repository.
type SelfServiceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	// AgentImage runs the session unless spec.agentImage is set.
	AgentImage string
	// AgentKey derives every agent's API token (see AgentToken).
	AgentKey string
	// HTTPClient calls git host APIs; nil means http.DefaultClient.
	HTTPClient *http.Client
}

// +kubebuilder:rbac:groups=koptan.felukka.org,resources=selfservices,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=koptan.felukka.org,resources=selfservices/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=koptan.felukka.org,resources=selfservices/finalizers,verbs=update
// +kubebuilder:rbac:groups=koptan.felukka.org,resources=services,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch

func (r *SelfServiceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var ss koptanv1.SelfService
	if err := r.Get(ctx, req.NamespacedName, &ss); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if !ss.DeletionTimestamp.IsZero() {
		if controllerutil.RemoveFinalizer(&ss, selfServiceFinalizer) {
			return ctrl.Result{}, r.Update(ctx, &ss)
		}
		return ctrl.Result{}, nil
	}
	if controllerutil.AddFinalizer(&ss, selfServiceFinalizer) {
		if err := r.Update(ctx, &ss); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	orig := ss.DeepCopy()
	res, err := r.provision(ctx, &ss)
	if err != nil {
		markSelfServiceFailed(&ss, err)
		res = ctrl.Result{RequeueAfter: failedRetryInterval}
	}
	ss.Status.ObservedGeneration = ss.Generation
	if perr := r.Status().Patch(ctx, &ss, client.MergeFrom(orig)); perr != nil {
		return ctrl.Result{}, perr
	}
	return res, nil
}

// provision makes every part exist and reports how far the session is.
func (r *SelfServiceReconciler) provision(ctx context.Context, ss *koptanv1.SelfService) (ctrl.Result, error) {
	if r.AgentKey == "" {
		return ctrl.Result{}, fmt.Errorf("the operator has no agent key (KOPTAN_AGENT_KEY); SelfServices are disabled")
	}
	repoURL, err := r.ensureRepo(ctx, ss)
	if err != nil {
		setSelfServiceCondition(ss, "RepoReady", metav1.ConditionFalse, "RepoUnavailable", err.Error())
		return ctrl.Result{}, err
	}
	ss.Status.RepoURL = repoURL
	setSelfServiceCondition(ss, "RepoReady", metav1.ConditionTrue, "Ready", "Repository "+repoURL)

	if err := r.ensureAgentSecret(ctx, ss); err != nil {
		return ctrl.Result{}, err
	}
	deployment, err := r.ensureAgent(ctx, ss, repoURL)
	if err != nil {
		return ctrl.Result{}, err
	}
	if err := r.ensureAgentService(ctx, ss); err != nil {
		return ctrl.Result{}, err
	}
	ss.Status.AgentService = agentName(ss)
	if err := r.ensureService(ctx, ss, repoURL); err != nil {
		return ctrl.Result{}, err
	}
	ss.Status.ServiceRef = ss.Name

	switch {
	case ss.Spec.Suspend:
		setSelfServiceCondition(ss, "AgentReady", metav1.ConditionFalse, "Suspended", "The agent is scaled to zero")
		setSelfServicePhase(ss, koptanv1.SelfServicePhaseReady, "Suspended", "Suspended; the Service keeps running")
	case deployment.Status.AvailableReplicas > 0:
		setSelfServiceCondition(ss, "AgentReady", metav1.ConditionTrue, "Available", "The agent accepts prompts")
		setSelfServicePhase(ss, koptanv1.SelfServicePhaseReady, "Ready", "The agent accepts prompts")
	default:
		setSelfServiceCondition(ss, "AgentReady", metav1.ConditionFalse, "Starting", "Waiting for the agent pod")
		setSelfServicePhase(ss, koptanv1.SelfServicePhaseProvisioning, "Starting", "Waiting for the agent pod")
		return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
	}
	return ctrl.Result{}, nil
}

func markSelfServiceFailed(ss *koptanv1.SelfService, err error) {
	setSelfServicePhase(ss, koptanv1.SelfServicePhaseFailed, "Failed", err.Error())
}

func setSelfServiceCondition(ss *koptanv1.SelfService, typ string, status metav1.ConditionStatus, reason, msg string) {
	meta.SetStatusCondition(&ss.Status.Conditions, metav1.Condition{
		Type: typ, Status: status, Reason: reason, Message: msg, ObservedGeneration: ss.Generation,
	})
}

// setSelfServicePhase records the overall state in the phase and Ready.
func setSelfServicePhase(ss *koptanv1.SelfService, phase koptanv1.SelfServicePhase, reason, msg string) {
	ss.Status.Phase, ss.Status.Message = phase, msg
	status := metav1.ConditionFalse
	if phase == koptanv1.SelfServicePhaseReady {
		status = metav1.ConditionTrue
	}
	setSelfServiceCondition(ss, "Ready", status, reason, msg)
}

// SetupWithManager sets up the controller with the Manager.
func (r *SelfServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&koptanv1.SelfService{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.Secret{}).
		Owns(&koptanv1.Service{}).
		Named("selfservice").
		Complete(r)
}
