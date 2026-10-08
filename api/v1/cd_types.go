package v1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CDPhase string

const (
	CDPhaseWaiting   CDPhase = "Waiting"
	CDPhaseDeploying CDPhase = "Deploying"
	CDPhaseRunning   CDPhase = "Running"
	CDPhaseFailed    CDPhase = "Failed"
)

// Resources defines resource requests/limits for the deployment.
type Resources struct {
	// +optional
	CPURequest *resource.Quantity `json:"cpuRequest,omitempty"`

	// +optional
	CPULimit *resource.Quantity `json:"cpuLimit,omitempty"`

	// +optional
	MemoryRequest *resource.Quantity `json:"memoryRequest,omitempty"`

	// +optional
	MemoryLimit *resource.Quantity `json:"memoryLimit,omitempty"`
}

// CDSpec defines the desired state of a CD deployment.
type CDSpec struct {
	// CI references the CI CRD that provides the built image.
	// +required
	CI NamespacedObjectReference `json:"ci"`

	// +kubebuilder:default=1
	// +optional
	Replicas int32 `json:"replicas,omitempty"`

	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// +optional
	Resources *Resources `json:"resources,omitempty"`
}

// CDStatus defines the observed state of a CD deployment.
type CDStatus struct {
	Phase      CDPhase            `json:"phase,omitempty"`
	Revision   string             `json:"latestRevision,omitempty"`
	Image      string             `json:"latestImage,omitempty"`
	Message    string             `json:"message,omitempty"`
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Image",type=string,JSONPath=`.status.latestImage`
// +kubebuilder:printcolumn:name="Revision",type=string,JSONPath=`.status.latestRevision`
// +kubebuilder:printcolumn:name="CI",type=string,JSONPath=`.spec.ci.name`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

type CD struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitzero"`
	Spec              CDSpec   `json:"spec"`
	Status            CDStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

type CDList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []CD `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CD{}, &CDList{})
}
