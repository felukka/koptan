package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:validation:Enum=Pending;Discovering;Building;Ready;Failed
type ServicePhase string

const (
	ServicePhasePending     ServicePhase = "Pending"
	ServicePhaseDiscovering ServicePhase = "Discovering"
	ServicePhaseBuilding    ServicePhase = "Building"
	ServicePhaseReady       ServicePhase = "Ready"
	ServicePhaseFailed      ServicePhase = "Failed"
)

type Source struct {
	// +kubebuilder:validation:MinLength=1
	// +required
	Repo string `json:"repo"`

	// +kubebuilder:default=main
	// +optional
	Revision string `json:"revision,omitempty"`

	// +optional
	SecretRef *corev1.SecretKeySelector `json:"secretRef,omitempty"`
}

type ServiceSpec struct {
	// +required
	Source Source `json:"source"`

	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`
}

type ServiceStatus struct {
	// +optional
	Phase ServicePhase `json:"phase,omitempty"`

	// DiscoveredLanguage reveals what the AI decided if spec.language was empty
	// +optional
	ServiceType string `json:"serviceType,omitempty"`

	// LatestRevision is the SHA of the last processed commit from the source repo.
	// +optional
	LatestRevision string `json:"latestRevision,omitempty"`

	// LastPushDetected is the time the controller last detected a new commit.
	// +optional
	LastPushDetected *metav1.Time `json:"lastPushDetected,omitempty"`

	// CIRef is the name of the active CI CRD managing builds for this Service.
	// +optional
	CIRef string `json:"ciRef,omitempty"`

	// CDRef is the name of the active CD CRD managing deployment for this Service.
	// +optional
	CDRef string `json:"cdRef,omitempty"`

	// +optional
	Error string `json:"error,omitempty"`

	// +optional
	Message string `json:"message,omitempty"`

	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Language",type=string,JSONPath=`.status.serviceType`
// +kubebuilder:printcolumn:name="Revision",type=string,JSONPath=`.status.latestRevision`
// +kubebuilder:printcolumn:name="CI",type=string,JSONPath=`.status.ciRef`
// +kubebuilder:printcolumn:name="CD",type=string,JSONPath=`.status.cdRef`
// +kubebuilder:printcolumn:name="LastPush",type=date,JSONPath=`.status.lastPushDetected`,priority=1
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

type Service struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ServiceSpec   `json:"spec"`
	Status            ServiceStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

type ServiceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Service `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Service{}, &ServiceList{})
}
