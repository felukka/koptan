package controller

import (
	"context"
	"net/http"
	"time"

	koptanv1 "github.com/felukka/koptan/api/v1"
	"github.com/felukka/koptan/internal/alerting"
	"github.com/felukka/koptan/internal/notify"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	// alertRetryInterval paces retries of failed deliveries and of
	// channels whose Secret cannot be read.
	alertRetryInterval = 30 * time.Second
	maxDeliveries      = 20
	notifyTimeout      = 10 * time.Second
)

// AlertReconciler sends notifications when a selected Service's pipeline
// changes: a new revision, a build starting, succeeding or failing, a
// rollout starting, succeeding or failing. It reads state, never changes
// it, so it needs no finalizer.
type AlertReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	// HTTPClient posts notifications; nil means notify.NewHTTPClient, which
	// refuses loopback and link-local destinations.
	HTTPClient *http.Client
}

// +kubebuilder:rbac:groups=koptan.felukka.org,resources=alerts,verbs=get;list;watch
// +kubebuilder:rbac:groups=koptan.felukka.org,resources=alerts/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=koptan.felukka.org,resources=services;cis;cds,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

func (r *AlertReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var alert koptanv1.Alert
	if err := r.Get(ctx, req.NamespacedName, &alert); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	orig := alert.DeepCopy()

	services, err := r.targets(ctx, &alert)
	if err != nil {
		return ctrl.Result{}, err
	}
	notifiers, err := r.notifiers(ctx, &alert)
	if err != nil {
		setAlertCondition(&alert, "Ready", metav1.ConditionFalse, "ChannelUnavailable", err.Error())
		return ctrl.Result{RequeueAfter: alertRetryInterval}, r.Status().Patch(ctx, &alert, client.MergeFrom(orig))
	}

	var occurrences []alerting.Occurrence
	selected := map[string]bool{}
	for i := range services {
		selected[services[i].Name] = true
		snap, err := r.snapshot(ctx, &services[i])
		if err != nil {
			return ctrl.Result{}, err
		}
		occurrences = append(occurrences, alerting.Events(snap)...)
	}

	sent := alerting.Prune(alert.Status.LastNotified, selected)
	decision := alerting.Plan(alerting.Input{
		Occurrences: occurrences,
		Channels:    channelNames(&alert),
		Wanted:      alert.Spec.Events,
		Sent:        sent,
		Baseline:    baselineChannels(&alert),
	})
	for k, v := range decision.Record {
		sent[k] = v
	}
	failed := r.deliver(ctx, &alert, notifiers, decision.Send, sent)

	alert.Status.LastNotified = sent
	alert.Status.Initialized = true
	alert.Status.ObservedGeneration = alert.Generation
	reportAlert(&alert, len(services), failed)
	if err := r.Status().Patch(ctx, &alert, client.MergeFrom(orig)); err != nil {
		return ctrl.Result{}, err
	}
	if failed > 0 {
		return ctrl.Result{RequeueAfter: alertRetryInterval}, nil
	}
	return ctrl.Result{}, nil
}

// deliver sends each delivery and records it; it returns how many failed.
// A failed delivery is not recorded as sent, so the next pass retries it.
func (r *AlertReconciler) deliver(ctx context.Context, alert *koptanv1.Alert, notifiers map[string]notify.Notifier,
	deliveries []alerting.Delivery, sent map[string]string) int {
	log := logf.FromContext(ctx)
	failed := 0
	for _, d := range deliveries {
		now := metav1.Now()
		e := d.Event
		e.Time = now.Time
		record := koptanv1.AlertDelivery{Time: now, Service: e.Service, Event: d.Kind,
			Channel: d.Channel, Revision: e.Revision, Success: true}
		if err := notifiers[d.Channel].Send(ctx, e); err != nil {
			log.Info("notification failed", "channel", d.Channel, "event", d.Kind, "error", err.Error())
			record.Success, record.Error = false, err.Error()
			failed++
		} else {
			sent[d.StateKey] = d.Key
		}
		alert.Status.Deliveries = append([]koptanv1.AlertDelivery{record}, alert.Status.Deliveries...)
	}
	if len(alert.Status.Deliveries) > maxDeliveries {
		alert.Status.Deliveries = alert.Status.Deliveries[:maxDeliveries]
	}
	return failed
}

// baselineChannels are the channels that record the current state without
// sending: all of them for a new or suspended Alert, else channels added
// by a spec change that have no state yet.
func baselineChannels(alert *koptanv1.Alert) map[string]bool {
	out := map[string]bool{}
	known := alerting.ChannelsInState(alert.Status.LastNotified)
	specChanged := alert.Status.ObservedGeneration != alert.Generation
	for _, ch := range alert.Spec.Channels {
		if !alert.Status.Initialized || alert.Spec.Suspend || (specChanged && !known[ch.Name]) {
			out[ch.Name] = true
		}
	}
	return out
}

func channelNames(alert *koptanv1.Alert) []string {
	out := make([]string, 0, len(alert.Spec.Channels))
	for _, ch := range alert.Spec.Channels {
		out = append(out, ch.Name)
	}
	return out
}

func reportAlert(alert *koptanv1.Alert, services, failed int) {
	switch {
	case alert.Spec.Suspend:
		setAlertCondition(alert, "Ready", metav1.ConditionFalse, "Suspended", "Notifications are suspended")
	case services == 0:
		setAlertCondition(alert, "Ready", metav1.ConditionFalse, "NoService", "No Service matches the Alert")
	default:
		setAlertCondition(alert, "Ready", metav1.ConditionTrue, "Watching", "Watching the selected Services")
	}
	if failed > 0 {
		setAlertCondition(alert, "Delivered", metav1.ConditionFalse, "DeliveryFailed",
			"Some notifications failed and will be retried; see status.deliveries")
	} else {
		setAlertCondition(alert, "Delivered", metav1.ConditionTrue, "Delivered", "Every notification was delivered")
	}
}

func setAlertCondition(alert *koptanv1.Alert, typ string, status metav1.ConditionStatus, reason, msg string) {
	meta.SetStatusCondition(&alert.Status.Conditions, metav1.Condition{
		Type: typ, Status: status, Reason: reason, Message: msg, ObservedGeneration: alert.Generation,
	})
}

// alertsForObject enqueues the Alerts in the object's namespace that may
// select its Service; for a CI or CD that is the Service in its label.
func (r *AlertReconciler) alertsForObject(ctx context.Context, o client.Object) []ctrl.Request {
	service := o.GetName()
	if _, isService := o.(*koptanv1.Service); !isService {
		service = o.GetLabels()[labelService]
	}
	var list koptanv1.AlertList
	if service == "" || r.List(ctx, &list, client.InNamespace(o.GetNamespace())) != nil {
		return nil
	}
	reqs := make([]ctrl.Request, 0, len(list.Items))
	for _, a := range list.Items {
		if a.Spec.Selector == nil && a.Spec.ServiceRef != nil && a.Spec.ServiceRef.Name != service {
			continue
		}
		reqs = append(reqs, ctrl.Request{NamespacedName: types.NamespacedName{Namespace: a.Namespace, Name: a.Name}})
	}
	return reqs
}

// SetupWithManager sets up the controller with the Manager.
func (r *AlertReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.HTTPClient == nil {
		r.HTTPClient = notify.NewHTTPClient(notifyTimeout)
	}
	enqueue := handler.EnqueueRequestsFromMapFunc(r.alertsForObject)
	return ctrl.NewControllerManagedBy(mgr).
		For(&koptanv1.Alert{}).
		Watches(&koptanv1.Service{}, enqueue).
		Watches(&koptanv1.CI{}, enqueue).
		Watches(&koptanv1.CD{}, enqueue).
		Named("alert").
		Complete(r)
}
