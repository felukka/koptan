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

// NamespacedObjectReference is a reference to a namespaced Kubernetes object.
type NamespacedObjectReference struct {
	// +kubebuilder:validation:MinLength=1
	// +required
	Name string `json:"name"`
}

// RegistrySpec defines where to push the built image.
type RegistrySpec struct {
	// +kubebuilder:default="docker.io"
	// +kubebuilder:validation:MinLength=1
	// +required
	Registry string `json:"registry"`

	// +kubebuilder:validation:MinLength=1
	// +required
	Repo string `json:"repo"`

	// CredentialsSecret names a kubernetes.io/dockerconfigjson Secret used
	// to push (and pull) the image.
	// +optional
	CredentialsSecret string `json:"credentialsSecret,omitempty"`

	// Deprecated: use CredentialsSecret. When set, the operator converts it
	// into a dockerconfigjson Secret named <ci>-registry.
	// +optional
	Creds *LoginSecret `json:"loginSecret,omitempty"`
}

// LoginSecret contains registry credentials.
type LoginSecret struct {
	// +required
	Username string `json:"username"`

	// base64 encoded secret
	// +required
	Password []byte `json:"password"`
}

// CISpec defines the desired state of a CI build.
type CISpec struct {
	// Service references the Service CRD that provides the source repo.
	// +required
	Service NamespacedObjectReference `json:"service"`

	// Image defines the target registry for the built image.
	// +required
	Registry RegistrySpec `json:"image"`

	// Revision is the commit SHA to build; the Service controller sets it.
	// +optional
	Revision string `json:"revision,omitempty"`

	// DockerfileConfigMap holds the Dockerfile (key "Dockerfile") and,
	// optionally, a default .dockerignore (key ".dockerignore").
	// +optional
	DockerfileConfigMap string `json:"dockerfileConfigMap,omitempty"`

	// ContextDir is the build context relative to the repository root.
	// +kubebuilder:validation:MaxLength=250
	// +kubebuilder:validation:Pattern=`^[A-Za-z0-9._-]+(/[A-Za-z0-9._-]+)*$`
	// +optional
	ContextDir string `json:"contextDir,omitempty"`

	// Plugins run, in order, after checkout and before build/push. The
	// Service controller resolves them; the generation makes a plugin
	// change rebuild.
	// +optional
	Plugins []CIPluginRef `json:"plugins,omitempty"`

	// +optional
	ExtraSteps []corev1.Container `json:"extraSteps,omitempty"`
}

// CIPluginRef pins a CIPlugin at the generation the build should use.
type CIPluginRef struct {
	// +kubebuilder:validation:MinLength=1
	// +required
	Name string `json:"name"`
	// +optional
	Generation int64 `json:"generation,omitempty"`
}

// PluginPhase is the state of one plugin step in the latest build.
type PluginPhase string

const (
	PluginPhasePending   PluginPhase = "Pending"
	PluginPhaseRunning   PluginPhase = "Running"
	PluginPhaseSucceeded PluginPhase = "Succeeded"
	PluginPhaseFailed    PluginPhase = "Failed"
)

// PluginResult is the outcome of one plugin step in the latest build.
type PluginResult struct {
	Name  string      `json:"name"`
	Phase PluginPhase `json:"phase"`
	// +optional
	Message string `json:"message,omitempty"`
}

// CIStatus defines the observed state of a CI build.
type CIStatus struct {
	Phase      CIPhase      `json:"phase,omitempty"`
	Revision   string       `json:"latestRevision,omitempty"`
	Image      string       `json:"latestImage,omitempty"`
	BuildCount int64        `json:"buildCount,omitempty"`
	BuildTime  *metav1.Time `json:"lastBuildTime,omitempty"`
	Message    string       `json:"message,omitempty"`
	// BuildPod is the Pod running (or that ran) the latest build.
	// +optional
	BuildPod string `json:"buildPod,omitempty"`
	// BuildingRevision is the revision of the build in BuildPod.
	// +optional
	BuildingRevision string `json:"buildingRevision,omitempty"`
	// PluginResults are the plugin steps of the latest build, in order.
	// +optional
	PluginResults []PluginResult `json:"pluginResults,omitempty"`

	// ObservedGeneration is the spec generation of the latest build.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Image",type=string,JSONPath=`.status.latestImage`
// +kubebuilder:printcolumn:name="Revision",type=string,JSONPath=`.status.latestRevision`
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
