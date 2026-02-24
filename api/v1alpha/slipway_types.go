package v1alpha

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type AppRef struct {
	// +kubebuilder:validation:MinLength=1
	// +required
	Name string `json:"name"`

	// +kubebuilder:validation:Enum=GoApp;DotnetApp;JavaApp
	// +required
	Kind string `json:"kind"`
}

type ImageSpec struct {
	// +kubebuilder:default="docker.io"
	// +kubebuilder:validation:MinLength=1
	// +required
	Registry string `json:"registry"`

	// +kubebuilder:validation:MinLength=1
	// +required
	Name string `json:"name"`

	// +optional
	Creds *RegistryCreds `json:"creds,omitempty"`
}

type RegistryCreds struct {
	// +required
	Username string `json:"username"`
	// +required
	Password string `json:"password"`
}

type PollSpec struct {
	// +kubebuilder:default=60
	// +kubebuilder:validation:Minimum=10
	// +optional
	IntervalSeconds int64 `json:"intervalSeconds,omitempty"`
}

type SlipwaySpec struct {
	// +required
	AppRef AppRef `json:"appRef"`

	// +required
	Image ImageSpec `json:"image"`

	// +optional
	Poll PollSpec `json:"poll,omitempty"`

	// +optional
	ExtraSteps []corev1.Container `json:"extraSteps,omitempty"`
}

type SlipwayPhase string

const (
	SlipwayPhaseIdle      SlipwayPhase = "Idle"
	SlipwayPhaseResolving SlipwayPhase = "Resolving"
	SlipwayPhaseBuilding  SlipwayPhase = "Building"
	SlipwayPhaseSucceeded SlipwayPhase = "Succeeded"
	SlipwayPhaseFailed    SlipwayPhase = "Failed"
)

type SlipwayStatus struct {
	Phase          SlipwayPhase `json:"phase,omitempty"`
	LatestRevision string       `json:"latestRevision,omitempty"`
	LatestImage    string       `json:"latestImage,omitempty"`
	BuildCount     int64        `json:"buildCount,omitempty"`
	LastBuildTime  *metav1.Time `json:"lastBuildTime,omitempty"`
	LastPollTime   *metav1.Time `json:"lastPollTime,omitempty"`
	Message        string       `json:"message,omitempty"`
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Image",type=string,JSONPath=`.status.latestImage`
// +kubebuilder:printcolumn:name="Revision",type=string,JSONPath=`.status.latestRevision`,priority=1
// +kubebuilder:printcolumn:name="Builds",type=integer,JSONPath=`.status.buildCount`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

type Slipway struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitzero"`
	Spec              SlipwaySpec   `json:"spec"`
	Status            SlipwayStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

type SlipwayList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []Slipway `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Slipway{}, &SlipwayList{})
}
