package v1alpha

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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

func (r *Resources) ToK8s() corev1.ResourceRequirements {
	reqs := corev1.ResourceRequirements{}
	if r == nil {
		return reqs
	}
	if r.CPURequest != nil || r.MemoryRequest != nil {
		reqs.Requests = corev1.ResourceList{}
		if r.CPURequest != nil {
			reqs.Requests[corev1.ResourceCPU] = *r.CPURequest
		}
		if r.MemoryRequest != nil {
			reqs.Requests[corev1.ResourceMemory] = *r.MemoryRequest
		}
	}
	if r.CPULimit != nil || r.MemoryLimit != nil {
		reqs.Limits = corev1.ResourceList{}
		if r.CPULimit != nil {
			reqs.Limits[corev1.ResourceCPU] = *r.CPULimit
		}
		if r.MemoryLimit != nil {
			reqs.Limits[corev1.ResourceMemory] = *r.MemoryLimit
		}
	}
	return reqs
}

type HealthCheck struct {
	// +optional
	Path string `json:"path,omitempty"`

	// +optional
	Port int32 `json:"port,omitempty"`

	// +kubebuilder:default=30
	// +optional
	InitialDelaySeconds int32 `json:"initialDelaySeconds,omitempty"`

	// +kubebuilder:default=10
	// +optional
	PeriodSeconds int32 `json:"periodSeconds,omitempty"`
}

type VoyageSpec struct {
	// +required
	SlipwayRef SlipwayRef `json:"slipwayRef"`

	// +required
	Port int32 `json:"port"`

	// +kubebuilder:default=1
	// +optional
	Replicas int32 `json:"replicas,omitempty"`

	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// +optional
	Resources *Resources `json:"resources,omitempty"`

	// +optional
	HealthCheck *HealthCheck `json:"healthCheck,omitempty"`
}

type SlipwayRef struct {
	Name string `json:"name"`
}

type VoyagePhase string

const (
	VoyagePhaseWaiting   VoyagePhase = "Waiting"
	VoyagePhaseDeploying VoyagePhase = "Deploying"
	VoyagePhaseRunning   VoyagePhase = "Running"
	VoyagePhaseFailed    VoyagePhase = "Failed"
)

type VoyageStatus struct {
	Phase         VoyagePhase        `json:"phase,omitempty"`
	DeployedImage string             `json:"deployedImage,omitempty"`
	Conditions    []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Image",type=string,JSONPath=`.status.deployedImage`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

type Voyage struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitzero"`
	Spec              VoyageSpec   `json:"spec"`
	Status            VoyageStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

type VoyageList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []Voyage `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Voyage{}, &VoyageList{})
}
