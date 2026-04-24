package v1alpha

// +kubebuilder:validation:Enum=Pending;Discovering;Ready;Failed
type AppPhase string

const (
	AppPhasePending     AppPhase = "Pending"
	AppPhaseDiscovering AppPhase = "Discovering"
	AppPhaseReady       AppPhase = "Ready"
	AppPhaseFailed      AppPhase = "Failed"
)

const (
	AppConditionDockerfileGenerated = "DockerfileGenerated"
	AppConditionConfigMapReady      = "ConfigMapReady"
)

type SourceRef struct {
	// +kubebuilder:validation:MinLength=1
	// +required
	Repo string `json:"repo"`

	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:default=main
	// +optional
	Revision string `json:"revision,omitempty"`

	// +optional
	PATToken string `json:"patToken,omitempty"`

	// +optional
	DockerfileName string `json:"dockerfileName,omitempty"`
}
