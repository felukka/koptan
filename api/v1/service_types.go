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

	// Port the application listens on; also exported as $PORT.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +optional
	Port int32 `json:"port,omitempty"`
}

type ServiceStatus struct {
	// +optional
	Phase ServicePhase `json:"phase,omitempty"`

	// DiscoveredLanguage reveals what the AI decided if spec.language was empty
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
