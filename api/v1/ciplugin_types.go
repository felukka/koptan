package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CIPluginType selects the built-in renderer for a plugin.
// +kubebuilder:validation:Enum=sonarqube;codeql;custom
type CIPluginType string

const (
	CIPluginSonarQube CIPluginType = "sonarqube"
	CIPluginCodeQL    CIPluginType = "codeql"
	CIPluginCustom    CIPluginType = "custom"
)

// FailurePolicy says what a failing step does to the build.
// +kubebuilder:validation:Enum=Fail;Ignore
type FailurePolicy string

const (
	FailurePolicyFail   FailurePolicy = "Fail"
	FailurePolicyIgnore FailurePolicy = "Ignore"
)

// PluginTargetRef attaches a plugin to a Service in the plugin's namespace.
type PluginTargetRef struct {
	// +kubebuilder:validation:Enum=Service
	// +kubebuilder:default=Service
	// +optional
	Kind string `json:"kind,omitempty"`

	// +kubebuilder:validation:MinLength=1
	// +required
	Name string `json:"name"`
}

// SonarQubeConfig runs sonar-scanner against a SonarQube server.
type SonarQubeConfig struct {
	// HostURL of the SonarQube server.
	// +kubebuilder:validation:Pattern=`^https?://[^\s]+$`
	// +required
	HostURL string `json:"hostURL"`

	// TokenSecretRef holds the analysis token.
	// +required
	TokenSecretRef corev1.SecretKeySelector `json:"tokenSecretRef"`

	// ProjectKey defaults to <namespace>_<service>.
	// +kubebuilder:validation:MaxLength=400
	// +optional
	ProjectKey string `json:"projectKey,omitempty"`

	// WaitForQualityGate fails the step when the quality gate fails.
	// +kubebuilder:default=true
	// +optional
	WaitForQualityGate *bool `json:"waitForQualityGate,omitempty"`

	// ExtraArgs are passed to sonar-scanner, e.g. -Dsonar.exclusions=**/*_test.go.
	// +optional
	ExtraArgs []string `json:"extraArgs,omitempty"`

	// Image overrides the scanner image.
	// +optional
	Image string `json:"image,omitempty"`
}

// CodeQLSeverity is the lowest result level that fails the step.
// +kubebuilder:validation:Enum=error;warning;note;none
type CodeQLSeverity string

// CodeQLGitHubUpload uploads the SARIF results to GitHub code scanning.
type CodeQLGitHubUpload struct {
	// Repository is owner/name on GitHub.
	// +kubebuilder:validation:Pattern=`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`
	// +required
	Repository string `json:"repository"`

	// Ref the results belong to, e.g. refs/heads/main.
	// +kubebuilder:validation:Pattern=`^refs/[A-Za-z0-9._/-]+$`
	// +required
	Ref string `json:"ref"`

	// TokenSecretRef holds a token with security_events scope.
	// +required
	TokenSecretRef corev1.SecretKeySelector `json:"tokenSecretRef"`
}

// CodeQLConfig analyzes the source with the CodeQL CLI.
type CodeQLConfig struct {
	// Languages to analyze, e.g. javascript-typescript, python, java,
	// csharp, ruby, go. Empty means the language Koptan detected.
	// +optional
	Languages []string `json:"languages,omitempty"`

	// QuerySuite is code-scanning, security-extended or security-and-quality.
	// +kubebuilder:validation:Enum=code-scanning;security-extended;security-and-quality
	// +kubebuilder:default=code-scanning
	// +optional
	QuerySuite string `json:"querySuite,omitempty"`

	// FailOnSeverity fails the step when a result at or above it is found.
	// +kubebuilder:default=error
	// +optional
	FailOnSeverity CodeQLSeverity `json:"failOnSeverity,omitempty"`

	// BundleURL overrides the CodeQL bundle download.
	// +kubebuilder:validation:Pattern=`^https://[^\s]+$`
	// +optional
	BundleURL string `json:"bundleURL,omitempty"`

	// Image runs the analysis; it must be glibc-based. Languages that need
	// a build (go, cpp) need their toolchain in it.
	// +optional
	Image string `json:"image,omitempty"`

	// +optional
	GitHub *CodeQLGitHubUpload `json:"github,omitempty"`
}

// CustomStep runs any container against the checked-out source.
type CustomStep struct {
	// +kubebuilder:validation:MinLength=1
	// +required
	Image string `json:"image"`
	// +optional
	Command []string `json:"command,omitempty"`
	// +optional
	Args []string `json:"args,omitempty"`
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`
	// +optional
	EnvFrom []corev1.EnvFromSource `json:"envFrom,omitempty"`
}

// CIPluginSpec is a step that runs after checkout and before build/push.
// +kubebuilder:validation:XValidation:rule="self.type != 'sonarqube' || has(self.sonarqube)",message="type sonarqube needs spec.sonarqube"
// +kubebuilder:validation:XValidation:rule="self.type != 'codeql' || has(self.codeql)",message="type codeql needs spec.codeql"
// +kubebuilder:validation:XValidation:rule="self.type != 'custom' || has(self.custom)",message="type custom needs spec.custom"
type CIPluginSpec struct {
	// +required
	Type CIPluginType `json:"type"`

	// Order sorts the steps of a build, lowest first; ties sort by name.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=1000
	// +kubebuilder:default=100
	// +optional
	Order int32 `json:"order,omitempty"`

	// FailurePolicy Ignore lets the build go on when the step fails.
	// +kubebuilder:default=Fail
	// +optional
	FailurePolicy FailurePolicy `json:"failurePolicy,omitempty"`

	// TargetRefs attach the plugin to Services by name, in addition to
	// Services that list it in spec.plugins.
	// +kubebuilder:validation:MaxItems=64
	// +optional
	TargetRefs []PluginTargetRef `json:"targetRefs,omitempty"`

	// Selector attaches the plugin to every Service whose labels match.
	// +optional
	Selector *metav1.LabelSelector `json:"selector,omitempty"`

	// Resources of the step container.
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// +optional
	SonarQube *SonarQubeConfig `json:"sonarqube,omitempty"`
	// +optional
	CodeQL *CodeQLConfig `json:"codeql,omitempty"`
	// +optional
	Custom *CustomStep `json:"custom,omitempty"`
}

// CIPluginStatus reports whether the plugin is usable and where it runs.
type CIPluginStatus struct {
	// AttachedServices lists the Services whose builds run this plugin.
	// +optional
	AttachedServices []string `json:"attachedServices,omitempty"`

	// ObservedGeneration is the spec generation the status describes.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=cip
// +kubebuilder:printcolumn:name="Type",type=string,JSONPath=`.spec.type`
// +kubebuilder:printcolumn:name="Order",type=integer,JSONPath=`.spec.order`
// +kubebuilder:printcolumn:name="Accepted",type=string,JSONPath=`.status.conditions[?(@.type=="Accepted")].status`
// +kubebuilder:printcolumn:name="Services",type=string,JSONPath=`.status.attachedServices`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// CIPlugin is a pluggable CI step, such as a SonarQube or CodeQL scan.
type CIPlugin struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitzero"`
	Spec              CIPluginSpec   `json:"spec"`
	Status            CIPluginStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// CIPluginList contains a list of CIPlugin.
type CIPluginList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []CIPlugin `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CIPlugin{}, &CIPluginList{})
}
