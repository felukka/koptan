package v1alpha

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type DotnetAppSpec struct {
	// +required
	Source SourceRef `json:"source"`

	// +optional
	SDKVersion string `json:"sdkVersion,omitempty"`

	// +optional
	ProjectPath string `json:"projectPath,omitempty"`

	// +kubebuilder:default="Release"
	// +optional
	Configuration string `json:"configuration,omitempty"`

	// +kubebuilder:default=false
	// +optional
	SelfContained bool `json:"selfContained,omitempty"`

	// +optional
	ExtraNugetSources []string `json:"extraNugetSources,omitempty"`

	// +optional
	BuildArgs []string `json:"buildArgs,omitempty"`

	// +optional
	ExtraPackages []string `json:"extraPackages,omitempty"`

	// +optional
	Env map[string]string `json:"env,omitempty"`
}

type DotnetAppStatus struct {
	// +optional
	Phase AppPhase `json:"phase,omitempty"`

	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// +optional
	ConfigMapName string `json:"configMapName,omitempty"`

	// +optional
	DiscoveredSDKVersion string `json:"discoveredSDKVersion,omitempty"`

	// +optional
	DiscoveredProjectPath string `json:"discoveredProjectPath,omitempty"`

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
// +kubebuilder:printcolumn:name="ConfigMap",type=string,JSONPath=`.status.configMapName`
// +kubebuilder:printcolumn:name="SDKVersion",type=string,JSONPath=`.status.discoveredSDKVersion`
// +kubebuilder:printcolumn:name="Error",type=string,JSONPath=`.status.error`,priority=1
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

type DotnetApp struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitzero"`
	Spec              DotnetAppSpec   `json:"spec"`
	Status            DotnetAppStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

type DotnetAppList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []DotnetApp `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DotnetApp{}, &DotnetAppList{})
}
