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

	// +optional
	Error string `json:"error,omitempty"`

	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Auto-Lang",type=string,JSONPath=`.status.discoveredLanguage`
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
