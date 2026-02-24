package v1alpha

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type GoAppSpec struct {
	// +required
	Source SourceRef `json:"source"`

	// +optional
	GoVersion string `json:"goVersion,omitempty"`

	// +optional
	Entrypoint string `json:"entrypoint,omitempty"`

	// +kubebuilder:default=false
	// +optional
	CGOEnabled bool `json:"cgoEnabled,omitempty"`

	// +optional
	LDFlags string `json:"ldflags,omitempty"`

	// +optional
	BuildArgs []string `json:"buildArgs,omitempty"`

	// +optional
	ExtraPackages []string `json:"extraPackages,omitempty"`

	// +optional
	Env map[string]string `json:"env,omitempty"`
}

type GoAppStatus struct {
	// +optional
	Phase AppPhase `json:"phase,omitempty"`

	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// +optional
	ConfigMapName string `json:"configMapName,omitempty"`

	// +optional
	DiscoveredGoVersion string `json:"discoveredGoVersion,omitempty"`

	// +optional
	DiscoveredEntrypoint string `json:"discoveredEntrypoint,omitempty"`

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
// +kubebuilder:printcolumn:name="GoVersion",type=string,JSONPath=`.status.discoveredGoVersion`
// +kubebuilder:printcolumn:name="Error",type=string,JSONPath=`.status.error`,priority=1
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

type GoApp struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitzero"`
	Spec              GoAppSpec   `json:"spec"`
	Status            GoAppStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

type GoAppList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []GoApp `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GoApp{}, &GoAppList{})
}
