package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AlertEvent is a pipeline event an Alert can notify about.
// +kubebuilder:validation:Enum=Push;CIStarted;CISucceeded;CIFailed;CDDeploying;CDSucceeded;CDFailed
type AlertEvent string

const (
	AlertPush        AlertEvent = "Push"
	AlertCIStarted   AlertEvent = "CIStarted"
	AlertCISucceeded AlertEvent = "CISucceeded"
	AlertCIFailed    AlertEvent = "CIFailed"
	AlertCDDeploying AlertEvent = "CDDeploying"
	AlertCDSucceeded AlertEvent = "CDSucceeded"
	AlertCDFailed    AlertEvent = "CDFailed"
)

// ChannelType is where a notification goes.
// +kubebuilder:validation:Enum=slack;teams;webhook
type ChannelType string

const (
	ChannelSlack   ChannelType = "slack"
	ChannelTeams   ChannelType = "teams"
	ChannelWebhook ChannelType = "webhook"
)

// AlertChannel is one destination. The URL lives in a Secret because Slack
// and Teams webhook URLs are credentials.
type AlertChannel struct {
	// Name identifies the channel in the status.
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]{0,61}[a-z0-9])?$`
	// +required
	Name string `json:"name"`

	// +required
	Type ChannelType `json:"type"`

	// URLSecretRef holds the incoming webhook URL (https; plain http is
	// allowed for type webhook only).
	// +required
	URLSecretRef corev1.SecretKeySelector `json:"urlSecretRef"`

	// SigningSecretRef, for type webhook, signs each body with HMAC-SHA256
	// in the X-Koptan-Signature header (sha256=<hex>).
	// +optional
	SigningSecretRef *corev1.SecretKeySelector `json:"signingSecretRef,omitempty"`
}

// AlertSpec says which Services to watch, which events to send and where.
// +kubebuilder:validation:XValidation:rule="has(self.serviceRef) || has(self.selector)",message="set serviceRef or selector"
type AlertSpec struct {
	// ServiceRef selects one Service in the Alert's namespace.
	// +optional
	ServiceRef *NamespacedObjectReference `json:"serviceRef,omitempty"`

	// Selector selects Services in the Alert's namespace by label.
	// +optional
	Selector *metav1.LabelSelector `json:"selector,omitempty"`

	// Events to send; empty means all of them.
	// +kubebuilder:validation:MaxItems=7
	// +listType=set
	// +optional
	Events []AlertEvent `json:"events,omitempty"`

	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=10
	// +listType=map
	// +listMapKey=name
	// +required
	Channels []AlertChannel `json:"channels"`

	// Suspend stops sending; events that happen meanwhile are skipped.
	// +optional
	Suspend bool `json:"suspend,omitempty"`
}

// AlertDelivery records one notification attempt.
type AlertDelivery struct {
	Time     metav1.Time `json:"time"`
	Service  string      `json:"service"`
	Event    AlertEvent  `json:"event"`
	Channel  string      `json:"channel"`
	Revision string      `json:"revision,omitempty"`
	Success  bool        `json:"success"`
	// +optional
	Error string `json:"error,omitempty"`
}

// AlertStatus tracks what was sent, so nothing is sent twice.
type AlertStatus struct {
	// Initialized is set once the state at creation has been recorded;
	// an Alert reports what happens after it exists.
	// +optional
	Initialized bool `json:"initialized,omitempty"`

	// LastNotified maps <service>/<event>/<channel> to the key of the last
	// event handled, e.g. the commit SHA or the build Pod.
	// +optional
	LastNotified map[string]string `json:"lastNotified,omitempty"`

	// Deliveries are the most recent notification attempts, newest first.
	// +kubebuilder:validation:MaxItems=20
	// +optional
	Deliveries []AlertDelivery `json:"deliveries,omitempty"`

	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Service",type=string,JSONPath=`.spec.serviceRef.name`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Suspended",type=boolean,JSONPath=`.spec.suspend`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Alert sends Slack, Teams or webhook notifications about a Service's
// pushes, builds and deployments.
type Alert struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitzero"`
	Spec              AlertSpec   `json:"spec"`
	Status            AlertStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AlertList contains a list of Alert.
type AlertList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []Alert `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Alert{}, &AlertList{})
}
