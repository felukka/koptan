package v1alpha

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type JavaAppSpec struct {
	// +required
	Source SourceRef `json:"source"`

	// +optional
	JavaVersion string `json:"javaVersion,omitempty"`

	// +kubebuilder:validation:Enum=maven;gradle
	// +optional
	BuildTool string `json:"buildTool,omitempty"`

	// +optional
	ArtifactPath string `json:"artifactPath,omitempty"`

	// +kubebuilder:default="package"
	// +optional
	MavenGoal string `json:"mavenGoal,omitempty"`

	// +kubebuilder:default="build"
	// +optional
	GradleTask string `json:"gradleTask,omitempty"`

	// +optional
	MavenProfiles []string `json:"mavenProfiles,omitempty"`

	// +optional
	JVMArgs string `json:"jvmArgs,omitempty"`

	// +kubebuilder:default=false
	// +optional
	SpringBoot bool `json:"springBoot,omitempty"`

	// +optional
	BuildArgs []string `json:"buildArgs,omitempty"`

	// +optional
	ExtraPackages []string `json:"extraPackages,omitempty"`

	// +optional
	Env map[string]string `json:"env,omitempty"`
}

type JavaAppStatus struct {
	// +optional
	Phase AppPhase `json:"phase,omitempty"`

	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// +optional
	ConfigMapName string `json:"configMapName,omitempty"`

	// +optional
	DiscoveredJavaVersion string `json:"discoveredJavaVersion,omitempty"`

	// +optional
	DiscoveredBuildTool string `json:"discoveredBuildTool,omitempty"`

	// +optional
	DiscoveredArtifactPath string `json:"discoveredArtifactPath,omitempty"`

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
// +kubebuilder:printcolumn:name="BuildTool",type=string,JSONPath=`.status.discoveredBuildTool`
// +kubebuilder:printcolumn:name="Error",type=string,JSONPath=`.status.error`,priority=1
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

type JavaApp struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitzero"`
	Spec              JavaAppSpec   `json:"spec"`
	Status            JavaAppStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

type JavaAppList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []JavaApp `json:"items"`
}

func init() {
	SchemeBuilder.Register(&JavaApp{}, &JavaAppList{})
}
