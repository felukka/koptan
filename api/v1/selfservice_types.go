package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SelfServicePhase is the lifecycle phase of a SelfService.
// +kubebuilder:validation:Enum=Provisioning;Ready;Failed
type SelfServicePhase string

const (
	SelfServicePhaseProvisioning SelfServicePhase = "Provisioning"
	SelfServicePhaseReady        SelfServicePhase = "Ready"
	SelfServicePhaseFailed       SelfServicePhase = "Failed"
)

// GitProvider is a git host the operator can create repositories on.
// +kubebuilder:validation:Enum=github;gitlab
type GitProvider string

const (
	GitProviderGitHub GitProvider = "github"
	GitProviderGitLab GitProvider = "gitlab"
)

// ExistingRepo is a repository that already exists; it may be empty.
type ExistingRepo struct {
	// URL is the https clone URL.
	// +kubebuilder:validation:Pattern=`^https://[^\s@]+$`
	// +required
	URL string `json:"url"`

	// SecretRef holds a token that can push to the repository.
	// +required
	SecretRef corev1.SecretKeySelector `json:"secretRef"`
}

// CreateRepo has the operator create the repository.
type CreateRepo struct {
	// +required
	Provider GitProvider `json:"provider"`

	// Owner is the organization or user (GitHub), or the group path
	// (GitLab). Empty means the token's own account.
	// +kubebuilder:validation:Pattern=`^[A-Za-z0-9_.-]+(/[A-Za-z0-9_.-]+)*$`
	// +optional
	Owner string `json:"owner,omitempty"`

	// Name of the repository; defaults to the SelfService name.
	// +kubebuilder:validation:Pattern=`^[A-Za-z0-9_.-]{1,100}$`
	// +optional
	Name string `json:"name,omitempty"`

	// +kubebuilder:default=true
	// +optional
	Private *bool `json:"private,omitempty"`

	// BaseURL is the API of GitHub Enterprise or a self-hosted GitLab.
	// +kubebuilder:validation:Pattern=`^https://[^\s]+$`
	// +optional
	BaseURL string `json:"baseURL,omitempty"`

	// TokenSecretRef holds a token that can create repositories and push.
	// +required
	TokenSecretRef corev1.SecretKeySelector `json:"tokenSecretRef"`
}

// SelfServiceRepo says which repository the session works on.
// +kubebuilder:validation:XValidation:rule="has(self.existing) != has(self.create)",message="set exactly one of existing or create"
type SelfServiceRepo struct {
	// +optional
	Existing *ExistingRepo `json:"existing,omitempty"`
	// +optional
	Create *CreateRepo `json:"create,omitempty"`
}

// AgentAI configures the model the agent uses.
type AgentAI struct {
	// +kubebuilder:validation:Enum=anthropic;openai-compatible
	// +required
	Provider string `json:"provider"`

	// Model id, e.g. claude-opus-5-5 or llama3.1.
	// +kubebuilder:validation:MinLength=1
	// +required
	Model string `json:"model"`

	// BaseURL of an OpenAI-compatible server, e.g. http://ollama:11434/v1.
	// +optional
	BaseURL string `json:"baseURL,omitempty"`

	// APIKeySecretRef holds the API key; local servers usually need none.
	// +optional
	APIKeySecretRef *corev1.SecretKeySelector `json:"apiKeySecretRef,omitempty"`
}

// SelfServiceTemplate is the Service the SelfService creates for its repo.
type SelfServiceTemplate struct {
	// +optional
	Image *ImageSpec `json:"image,omitempty"`
	// +kubebuilder:validation:Minimum=0
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +optional
	Port int32 `json:"port,omitempty"`
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`
	// +optional
	Plugins []PluginRef `json:"plugins,omitempty"`
}

// SelfServiceSpec links a git repository to an agent session that turns
// prompts into commits; the repository deploys as a regular Service.
type SelfServiceSpec struct {
	// +required
	Repo SelfServiceRepo `json:"repo"`

	// Branch the agent commits to and the Service builds.
	// +kubebuilder:default=main
	// +kubebuilder:validation:Pattern=`^[A-Za-z0-9._/-]{1,200}$`
	// +optional
	Branch string `json:"branch,omitempty"`

	// +required
	AI AgentAI `json:"ai"`

	// +optional
	Service SelfServiceTemplate `json:"service,omitempty"`

	// AllowCommands lets the agent run shell commands (tests, formatters)
	// in its pod.
	// +optional
	AllowCommands bool `json:"allowCommands,omitempty"`

	// AgentImage overrides the operator's --agent-image.
	// +optional
	AgentImage string `json:"agentImage,omitempty"`

	// Suspend scales the agent to zero; the Service keeps running.
	// +optional
	Suspend bool `json:"suspend,omitempty"`
}

// SelfServiceStatus reports the repository, the agent and the Service.
type SelfServiceStatus struct {
	// +optional
	Phase SelfServicePhase `json:"phase,omitempty"`
	// RepoURL is the clone URL the agent and the Service use.
	// +optional
	RepoURL string `json:"repoURL,omitempty"`
	// AgentService is the in-cluster Service of the agent API (port 8080).
	// +optional
	AgentService string `json:"agentService,omitempty"`
	// ServiceRef is the koptan Service that builds and deploys the repo.
	// +optional
	ServiceRef string `json:"serviceRef,omitempty"`
	// +optional
	Message string `json:"message,omitempty"`
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=kss
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Repository",type=string,JSONPath=`.status.repoURL`
// +kubebuilder:printcolumn:name="Model",type=string,JSONPath=`.spec.ai.model`
// +kubebuilder:printcolumn:name="Service",type=string,JSONPath=`.status.serviceRef`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// SelfService is a git repository paired with an agent session: prompts
// from the UI become commits, which deploy like any Service.
type SelfService struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitzero"`
	Spec              SelfServiceSpec   `json:"spec"`
	Status            SelfServiceStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SelfServiceList contains a list of SelfService.
type SelfServiceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []SelfService `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SelfService{}, &SelfServiceList{})
}
