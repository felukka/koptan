package controller

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	koptanv1 "github.com/felukka/koptan/api/v1"
	"github.com/felukka/koptan/internal/gitprovider"
	"github.com/felukka/koptan/internal/utils"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	labelSelfService = "koptan.felukka.org/selfservice"
	agentPort        = 8080
	agentTokenKey    = "token"
	agentWorkspace   = "/workspace"
)

// AgentToken derives the bearer token of one agent from the shared key:
// hex(HMAC-SHA256(key, "<namespace>/<name>")). The Backstage backend
// derives the same token, so no Secret has to be read to call an agent,
// and a compromised agent only learns its own token.
func AgentToken(key, namespace, name string) string {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(namespace + "/" + name))
	return hex.EncodeToString(mac.Sum(nil))
}

func agentName(ss *koptanv1.SelfService) string { return ss.Name + "-agent" }

// gitSecretRef is the Secret with the token the agent pushes with.
func gitSecretRef(ss *koptanv1.SelfService) corev1.SecretKeySelector {
	if c := ss.Spec.Repo.Create; c != nil {
		return c.TokenSecretRef
	}
	return ss.Spec.Repo.Existing.SecretRef
}

// ensureRepo returns the clone URL, creating the repository when asked.
func (r *SelfServiceReconciler) ensureRepo(ctx context.Context, ss *koptanv1.SelfService) (string, error) {
	if e := ss.Spec.Repo.Existing; e != nil {
		return e.URL, validateHTTPSRepo(e.URL)
	}
	c := ss.Spec.Repo.Create
	if c == nil {
		return "", fmt.Errorf("spec.repo needs existing or create")
	}
	if ss.Status.RepoURL != "" && ss.Status.ObservedGeneration == ss.Generation {
		return ss.Status.RepoURL, nil
	}
	token, err := r.secretValue(ctx, ss.Namespace, c.TokenSecretRef)
	if err != nil {
		return "", err
	}
	provider, err := gitprovider.New(string(c.Provider), c.BaseURL, token, r.HTTPClient)
	if err != nil {
		return "", err
	}
	url, err := provider.EnsureRepo(ctx, gitprovider.RepoSpec{
		Owner:       c.Owner,
		Name:        firstNonEmpty(c.Name, ss.Name),
		Private:     c.Private == nil || *c.Private,
		Description: fmt.Sprintf("Created by Koptan for SelfService %s/%s", ss.Namespace, ss.Name),
	})
	if err != nil {
		return "", fmt.Errorf("create repository on %s: %w", c.Provider, err)
	}
	return url, validateHTTPSRepo(url)
}

// validateHTTPSRepo accepts what the agent can push to with a token.
func validateHTTPSRepo(url string) error {
	if err := utils.ValidateGitURL(url); err != nil {
		return err
	}
	if !strings.HasPrefix(url, "https://") {
		return fmt.Errorf("repository URL %q must use https", url)
	}
	return nil
}

func (r *SelfServiceReconciler) secretValue(ctx context.Context, ns string, ref corev1.SecretKeySelector) (string, error) {
	var secret corev1.Secret
	if err := r.Get(ctx, types.NamespacedName{Namespace: ns, Name: ref.Name}, &secret); err != nil {
		return "", fmt.Errorf("read secret %q: %w", ref.Name, err)
	}
	v, ok := secret.Data[ref.Key]
	if !ok || len(v) == 0 {
		return "", fmt.Errorf("secret %q has no key %q", ref.Name, ref.Key)
	}
	return string(v), nil
}

// ensureAgentSecret stores the agent's derived API token.
func (r *SelfServiceReconciler) ensureAgentSecret(ctx context.Context, ss *koptanv1.SelfService) error {
	s := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: agentName(ss), Namespace: ss.Namespace}}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, s, func() error {
		s.Labels = map[string]string{labelSelfService: ss.Name}
		s.Data = map[string][]byte{agentTokenKey: []byte(AgentToken(r.AgentKey, ss.Namespace, ss.Name))}
		return controllerutil.SetControllerReference(ss, s, r.Scheme)
	})
	return err
}

// ensureAgent runs the session: one replica (none when suspended) of the
// agent image, locked down, with the repository in an emptyDir.
func (r *SelfServiceReconciler) ensureAgent(ctx context.Context, ss *koptanv1.SelfService, repoURL string) (*appsv1.Deployment, error) {
	d := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: agentName(ss), Namespace: ss.Namespace}}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, d, func() error {
		labels := map[string]string{labelSelfService: ss.Name, "app.kubernetes.io/name": "koptan-agent"}
		d.Labels = labels
		d.Spec.Replicas = ptr.To(int32(1))
		if ss.Spec.Suspend {
			d.Spec.Replicas = ptr.To(int32(0))
		}
		d.Spec.Selector = &metav1.LabelSelector{MatchLabels: labels}
		d.Spec.Strategy = appsv1.DeploymentStrategy{Type: appsv1.RecreateDeploymentStrategyType}
		d.Spec.Template.Labels = labels
		d.Spec.Template.Spec = r.agentPodSpec(ss, repoURL)
		return controllerutil.SetControllerReference(ss, d, r.Scheme)
	})
	return d, err
}

func (r *SelfServiceReconciler) agentPodSpec(ss *koptanv1.SelfService, repoURL string) corev1.PodSpec {
	image := firstNonEmpty(ss.Spec.AgentImage, r.AgentImage)
	return corev1.PodSpec{
		AutomountServiceAccountToken: ptr.To(false),
		SecurityContext: &corev1.PodSecurityContext{
			RunAsNonRoot:   ptr.To(true),
			RunAsUser:      ptr.To(int64(1000)),
			RunAsGroup:     ptr.To(int64(1000)),
			FSGroup:        ptr.To(int64(1000)),
			SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
		},
		Containers: []corev1.Container{{
			Name:  "agent",
			Image: image,
			Args:  []string{"serve"},
			Ports: []corev1.ContainerPort{{Name: "http", ContainerPort: agentPort}},
			Env:   agentEnv(ss, repoURL),
			VolumeMounts: []corev1.VolumeMount{
				{Name: "workspace", MountPath: agentWorkspace},
				{Name: "tmp", MountPath: "/tmp"},
			},
			ReadinessProbe: &corev1.Probe{ProbeHandler: corev1.ProbeHandler{HTTPGet: &corev1.HTTPGetAction{
				Path: "/healthz", Port: intstr.FromInt32(agentPort)}}},
			SecurityContext: &corev1.SecurityContext{
				AllowPrivilegeEscalation: ptr.To(false),
				ReadOnlyRootFilesystem:   ptr.To(true),
				Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
			},
			Resources: resources("100m", "256Mi", "1", "1Gi"),
		}},
		Volumes: []corev1.Volume{
			{Name: "workspace", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
			{Name: "tmp", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
		},
	}
}

// agentEnv configures the CLI; credentials only ever come from Secrets.
func agentEnv(ss *koptanv1.SelfService, repoURL string) []corev1.EnvVar {
	secret := func(name string, ref corev1.SecretKeySelector) corev1.EnvVar {
		return corev1.EnvVar{Name: name, ValueFrom: &corev1.EnvVarSource{SecretKeyRef: ref.DeepCopy()}}
	}
	ai := ss.Spec.AI
	env := []corev1.EnvVar{
		{Name: "KOPTAN_AGENT_REPO", Value: repoURL},
		{Name: "KOPTAN_AGENT_BRANCH", Value: firstNonEmpty(ss.Spec.Branch, "main")},
		{Name: "KOPTAN_AGENT_PORT", Value: strconv.Itoa(agentPort)},
		{Name: "KOPTAN_AGENT_WORKSPACE", Value: agentWorkspace + "/repo"},
		{Name: "KOPTAN_AGENT_SERVICE", Value: ss.Name},
		{Name: "KOPTAN_AGENT_APP_PORT", Value: strconv.Itoa(int(templatePort(ss)))},
		{Name: "KOPTAN_AGENT_ALLOW_COMMANDS", Value: strconv.FormatBool(ss.Spec.AllowCommands)},
		{Name: "KOPTAN_AI_PROVIDER", Value: ai.Provider},
		{Name: "KOPTAN_AI_MODEL", Value: ai.Model},
		{Name: "KOPTAN_AI_BASE_URL", Value: ai.BaseURL},
		secret("KOPTAN_AGENT_TOKEN", corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: agentName(ss)}, Key: agentTokenKey}),
		secret("KOPTAN_GIT_TOKEN", gitSecretRef(ss)),
	}
	if ai.APIKeySecretRef != nil {
		env = append(env, secret("KOPTAN_AI_API_KEY", *ai.APIKeySecretRef))
	}
	return env
}

func templatePort(ss *koptanv1.SelfService) int32 {
	if ss.Spec.Service.Port != 0 {
		return ss.Spec.Service.Port
	}
	return defaultPort
}

// ensureAgentService exposes the agent API inside the cluster.
func (r *SelfServiceReconciler) ensureAgentService(ctx context.Context, ss *koptanv1.SelfService) error {
	s := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: agentName(ss), Namespace: ss.Namespace}}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, s, func() error {
		s.Labels = map[string]string{labelSelfService: ss.Name}
		s.Spec.Selector = map[string]string{labelSelfService: ss.Name, "app.kubernetes.io/name": "koptan-agent"}
		s.Spec.Ports = []corev1.ServicePort{{Name: "http", Port: agentPort, TargetPort: intstr.FromInt32(agentPort)}}
		return controllerutil.SetControllerReference(ss, s, r.Scheme)
	})
	return err
}

// ensureService creates the koptan Service that builds and deploys the
// repository. A Service of that name owned by something else is an error.
func (r *SelfServiceReconciler) ensureService(ctx context.Context, ss *koptanv1.SelfService, repoURL string) error {
	svc := &koptanv1.Service{ObjectMeta: metav1.ObjectMeta{Name: ss.Name, Namespace: ss.Namespace}}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, svc, func() error {
		if !svc.CreationTimestamp.IsZero() && !metav1.IsControlledBy(svc, ss) {
			return fmt.Errorf("a Service named %s already exists and does not belong to this SelfService", ss.Name)
		}
		if svc.Labels == nil {
			svc.Labels = map[string]string{}
		}
		svc.Labels[labelSelfService] = ss.Name
		ref := gitSecretRef(ss)
		t := ss.Spec.Service
		svc.Spec.Source = koptanv1.Source{Repo: repoURL, Revision: firstNonEmpty(ss.Spec.Branch, "main"), SecretRef: &ref}
		svc.Spec.Image, svc.Spec.Replicas, svc.Spec.Port = t.Image, t.Replicas, t.Port
		svc.Spec.Env, svc.Spec.Plugins = t.Env, t.Plugins
		return controllerutil.SetControllerReference(ss, svc, r.Scheme)
	})
	return err
}
