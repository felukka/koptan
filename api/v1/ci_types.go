package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CIPhase string

const (
	CIPhaseIdle      CIPhase = "Idle"
	CIPhaseResolving CIPhase = "Resolving"
	CIPhaseBuilding  CIPhase = "Building"
	CIPhaseSucceeded CIPhase = "Succeeded"
	CIPhaseFailed    CIPhase = "Failed"
)

type RegistrySpec struct {
	// +kubebuilder:default="docker.io"
	// +kubebuilder:validation:MinLength=1
	// +required
	Registry string `json:"registry"`

	// +kubebuilder:validation:MinLength=1
	// +required
	Repo string `json:"repo"`

	// +optional
	Creds *LoginSecret `json:"loginSecret,omitempty"`
}

type LoginSecret struct {
	// +required
	Username string `json:"username"`

	// base64 encoded secret
	// +required
	Password []byte `json:"password"`
}

type CISpec struct {
	// +required
	Service Service `json:"service"`

	// +required
	Registry RegistrySpec `json:"image"`

	// +optional
	ExtraSteps []corev1.Container `json:"extraSteps,omitempty"`
}

type CIStatus struct {
	Phase      CIPhase      `json:"phase,omitempty"`
	Revision   string       `json:"latestRevision,omitempty"`
	Image      string       `json:"latestImage,omitempty"`
	BuildCount int64        `json:"buildCount,omitempty"`
	BuildTime  *metav1.Time `json:"lastBuildTime,omitempty"`
	Message    string       `json:"message,omitempty"`
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Message",type=string,JSONPath=`.status.message`
// +kubebuilder:printcolumn:name="Image",type=string,JSONPath=`.status.latestImage`
// +kubebuilder:printcolumn:name="Revision",type=string,JSONPath=`.status.latestRevision`,priority=1
// +kubebuilder:printcolumn:name="Builds",type=integer,JSONPath=`.status.buildCount`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

type CI struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitzero"`
	Spec              CISpec   `json:"spec"`
	Status            CIStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

type CIList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []CI `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CI{}, &CIList{})
}
