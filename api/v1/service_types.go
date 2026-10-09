package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:validation:Enum=Pending;Discovering;Building;Ready;Failed
type ServicePhase string

const (
	ServicePhasePending     ServicePhase = "Pending"
	ServicePhaseDiscovering ServicePhase = "Discovering"
	ServicePhaseBuilding    ServicePhase = "Building"
	ServicePhaseReady       ServicePhase = "Ready"
	ServicePhaseFailed      ServicePhase = "Failed"
)

type Source struct {
	// +kubebuilder:validation:MinLength=1
	// +required
	Repo string `json:"repo"`

	// Revision is a branch, tag or full commit SHA. Empty means the
	// repository's default branch (remote HEAD).
	// +kubebuilder:validation:MaxLength=250
	// +kubebuilder:validation:Pattern=`^([A-Za-z0-9._/][A-Za-z0-9._/-]*)?$`
	// +optional
	Revision string `json:"revision,omitempty"`

	// +optional
	SecretRef *corev1.SecretKeySelector `json:"secretRef,omitempty"`
}

// ImageSpec says where the built image is pushed.
type ImageSpec struct {
	// Registry host, e.g. ghcr.io. Defaults to the operator's
	// --default-registry.
	// +optional
	Registry string `json:"registry,omitempty"`

	// Repo is the image repository inside the registry. Defaults to the
	// Service name.
	// +optional
	Repo string `json:"repo,omitempty"`

	// CredentialsSecret names a kubernetes.io/dockerconfigjson Secret used
	// to push the image and, on the Deployment, to pull it.
	// +optional
	CredentialsSecret string `json:"credentialsSecret,omitempty"`
}

// BuildSpec tunes how the image is built.
type BuildSpec struct {
	// ContextDir is the build context, relative to the repository root
	// (for a service inside a monorepo). Empty means the root.
	// +kubebuilder:validation:MaxLength=250
	// +kubebuilder:validation:Pattern=`^[A-Za-z0-9._-]+(/[A-Za-z0-9._-]+)*$`
	// +optional
	ContextDir string `json:"contextDir,omitempty"`

	// DockerfilePath is the repository's Dockerfile, relative to the
	// repository root. Empty means Dockerfile, Containerfile, docker/Dockerfile
	// or build/Dockerfile in the context, else a generated one.
	// +kubebuilder:validation:MaxLength=250
	// +kubebuilder:validation:Pattern=`^[A-Za-z0-9._-]+(/[A-Za-z0-9._-]+)*$`
	// +optional
	DockerfilePath string `json:"dockerfilePath,omitempty"`

	// Language skips detection and generates the Dockerfile for this stack.
	// +kubebuilder:validation:Enum=go;rust;java;dotnet;python;ruby;php;node;static
	// +optional
	Language string `json:"language,omitempty"`
}

// DetectedStack is what discovery learned about the repository.
type DetectedStack struct {
	Language string `json:"language,omitempty"`
	// +optional
	Version string `json:"version,omitempty"`
	// +optional
	PackageManager string `json:"packageManager,omitempty"`
	// +optional
	Framework string `json:"framework,omitempty"`
	// +optional
	Entrypoint string `json:"entrypoint,omitempty"`
}

type ServiceSpec struct {
	// +required
	Source Source `json:"source"`

	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// +optional
	Image *ImageSpec `json:"image,omitempty"`

	// +kubebuilder:validation:Minimum=0
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// +optional
	Build *BuildSpec `json:"build,omitempty"`

	// Port the application listens on; also exported as $PORT.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +optional
	Port int32 `json:"port,omitempty"`
}

type ServiceStatus struct {
	// +optional
	Phase ServicePhase `json:"phase,omitempty"`

	// ServiceType is the detected language, or "dockerfile" when only the
	// repository's own Dockerfile was found.
	// +optional
	ServiceType string `json:"serviceType,omitempty"`

	// LatestRevision is the SHA of the last processed commit from the source repo.
	// +optional
	LatestRevision string `json:"latestRevision,omitempty"`

	// LastPushDetected is the time the controller last detected a new commit.
	// +optional
	LastPushDetected *metav1.Time `json:"lastPushDetected,omitempty"`

	// CIRef is the name of the active CI CRD managing builds for this Service.
	// +optional
	CIRef string `json:"ciRef,omitempty"`

	// CDRef is the name of the active CD CRD managing deployment for this Service.
	// +optional
	CDRef string `json:"cdRef,omitempty"`

	// DockerfileConfigMap holds the Dockerfile CI builds with.
	// +optional
	DockerfileConfigMap string `json:"dockerfileConfigMap,omitempty"`

	// DockerfileSource says where the Dockerfile came from: the repository
	// or a template generated from the detected stack.
	// +kubebuilder:validation:Enum=repo;template
	// +optional
	DockerfileSource string `json:"dockerfileSource,omitempty"`

	// Detected describes the stack discovery found.
	// +optional
	Detected *DetectedStack `json:"detected,omitempty"`

	// ObservedGeneration is the spec generation the last discovery used.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// +optional
	Error string `json:"error,omitempty"`

	// +optional
	Message string `json:"message,omitempty"`

	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=ksvc
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Language",type=string,JSONPath=`.status.serviceType`
// +kubebuilder:printcolumn:name="Revision",type=string,JSONPath=`.status.latestRevision`
// +kubebuilder:printcolumn:name="CI",type=string,JSONPath=`.status.ciRef`
// +kubebuilder:printcolumn:name="CD",type=string,JSONPath=`.status.cdRef`
// +kubebuilder:printcolumn:name="LastPush",type=date,JSONPath=`.status.lastPushDetected`,priority=1
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

type Service struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ServiceSpec   `json:"spec"`
	Status            ServiceStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

type ServiceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Service `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Service{}, &ServiceList{})
}
