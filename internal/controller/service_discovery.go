package controller

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	koptanv1 "github.com/felukka/koptan/api/v1"
	"github.com/felukka/koptan/internal/service"
	"github.com/felukka/koptan/internal/utils"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	dockerfileKey   = "Dockerfile"
	dockerignoreKey = ".dockerignore"
)

// discoverAndBuild runs discovery at rev, stores the Dockerfile and points
// the CI at rev. A non-zero result or error means the Service is not Ready.
func (r *ServiceReconciler) discoverAndBuild(ctx context.Context, svc *koptanv1.Service,
	rev *utils.Revision, token string) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	setPhase(svc, koptanv1.ServicePhaseDiscovering,
		fmt.Sprintf("Discovering %s at %s", svc.Spec.Source.Repo, short(rev.SHA)))

	engine := r.Engine
	if engine == nil {
		engine = service.NewEngine()
	}
	result, err := engine.Discover(ctx, discoveryRequest(svc, rev, token))
	if err != nil {
		r.markFailed(svc, "DiscoveryFailed", fmt.Sprintf("discover: %v", err))
		return ctrl.Result{RequeueAfter: failedRetryInterval}, nil
	}
	log.Info("discovery complete", "language", result.Language(), "dockerfileSource", result.Source)

	cmName, err := r.ensureDockerfile(ctx, svc, result.Dockerfile, result.Dockerignore)
	if err != nil {
		r.markFailed(svc, "DockerfileFailed", fmt.Sprintf("store Dockerfile: %v", err))
		return ctrl.Result{}, err
	}

	ciName, err := r.ensureCI(ctx, svc, result.Language(), rev.SHA, cmName)
	if err != nil {
		if apierrors.IsConflict(err) || apierrors.IsAlreadyExists(err) {
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}
		r.markFailed(svc, "CIInitFailed", fmt.Sprintf("create CI: %v", err))
		return ctrl.Result{}, err
	}

	recordDiscovery(svc, result)
	svc.Status.LatestRevision = rev.SHA
	svc.Status.DockerfileConfigMap = cmName
	svc.Status.CIRef = ciName
	svc.Status.ObservedGeneration = svc.Generation
	svc.Status.Error = ""
	setPhase(svc, koptanv1.ServicePhaseReady,
		fmt.Sprintf("CI %s building %s", ciName, short(rev.SHA)))
	meta.SetStatusCondition(&svc.Status.Conditions, metav1.Condition{
		Type: "Ready", Status: metav1.ConditionTrue, Reason: "Discovered",
		Message: svc.Status.Message,
	})
	return ctrl.Result{}, nil
}

func discoveryRequest(svc *koptanv1.Service, rev *utils.Revision, token string) service.Request {
	req := service.Request{
		Repo:  svc.Spec.Source.Repo,
		Rev:   rev,
		Token: token,
		Port:  servicePort(svc),
	}
	if b := svc.Spec.Build; b != nil {
		req.ContextDir = b.ContextDir
		req.DockerfilePath = b.DockerfilePath
		req.Language = b.Language
	}
	return req
}

// recordDiscovery writes what discovery found into the status.
func recordDiscovery(svc *koptanv1.Service, result *service.Result) {
	svc.Status.ServiceType = result.Language()
	svc.Status.DockerfileSource = result.Source
	svc.Status.Detected = nil
	if f := result.Facts; f != nil {
		svc.Status.Detected = &koptanv1.DetectedStack{
			Language:       f.Language,
			Version:        f.Version,
			PackageManager: f.PackageManager,
			Framework:      f.Framework,
			Entrypoint:     f.Entrypoint,
		}
	}
	reason, message := "Repository", "Dockerfile taken from the repository"
	if result.Source == service.SourceTemplate {
		reason, message = "Template", "Dockerfile generated for "+result.Language()
	}
	meta.SetStatusCondition(&svc.Status.Conditions, metav1.Condition{
		Type: "DockerfileGenerated", Status: metav1.ConditionTrue, Reason: reason, Message: message,
	})
}

// ensureDockerfile stores the Dockerfile (and an optional .dockerignore) in
// ConfigMap <svc>-dockerfile-<hash>. The hash in the name changes the CI
// spec when the content changes, so CI rebuilds; older ConfigMaps are removed.
func (r *ServiceReconciler) ensureDockerfile(ctx context.Context, svc *koptanv1.Service,
	dockerfile, dockerignore []byte) (string, error) {
	h := sha256.New()
	h.Write(dockerfile)
	h.Write([]byte{0})
	h.Write(dockerignore)
	sum := h.Sum(nil)
	cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
		Name: fmt.Sprintf("%s-dockerfile-%x", svc.Name, sum[:4]), Namespace: svc.Namespace,
	}}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, cm, func() error {
		if cm.Labels == nil {
			cm.Labels = map[string]string{}
		}
		cm.Labels[labelService] = svc.Name
		cm.Data = map[string]string{dockerfileKey: string(dockerfile)}
		if len(dockerignore) > 0 {
			cm.Data[dockerignoreKey] = string(dockerignore)
		}
		return controllerutil.SetControllerReference(svc, cm, r.Scheme)
	}); err != nil {
		return "", err
	}
	var old corev1.ConfigMapList
	if err := r.List(ctx, &old, client.InNamespace(svc.Namespace),
		client.MatchingLabels{labelService: svc.Name}); err != nil {
		return "", err
	}
	for i := range old.Items {
		if old.Items[i].Name != cm.Name && metav1.IsControlledBy(&old.Items[i], svc) {
			if err := r.Delete(ctx, &old.Items[i]); client.IgnoreNotFound(err) != nil {
				return "", err
			}
		}
	}
	return cm.Name, nil
}

// ensureCI creates or updates the CI that builds this Service. An existing
// CI for the Service (e.g. one applied by hand) is reused.
func (r *ServiceReconciler) ensureCI(ctx context.Context, svc *koptanv1.Service,
	language, sha, dockerfileCM string) (string, error) {
	name := svc.Name + "-ci"
	var ciList koptanv1.CIList
	if err := r.List(ctx, &ciList, client.InNamespace(svc.Namespace)); err != nil {
		return "", err
	}
	for _, c := range ciList.Items {
		if c.Spec.Service.Name == svc.Name {
			if !c.DeletionTimestamp.IsZero() {
				return "", apierrors.NewConflict(koptanv1.GroupVersion.WithResource("cis").GroupResource(),
					c.Name, fmt.Errorf("CI is being deleted"))
			}
			name = c.Name
			break
		}
	}

	ci := &koptanv1.CI{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: svc.Namespace}}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, ci, func() error {
		if ci.Labels == nil {
			ci.Labels = map[string]string{}
		}
		ci.Labels[labelService] = svc.Name
		ci.Labels[labelLanguage] = language
		ci.Spec.Service.Name = svc.Name
		ci.Spec.Revision = sha
		ci.Spec.DockerfileConfigMap = dockerfileCM
		ci.Spec.ContextDir = ""
		if svc.Spec.Build != nil {
			ci.Spec.ContextDir = svc.Spec.Build.ContextDir
		}
		r.applyRegistry(svc, ci)
		if metav1.GetControllerOf(ci) == nil {
			return controllerutil.SetControllerReference(svc, ci, r.Scheme)
		}
		return nil
	})
	return name, err
}

// applyRegistry copies spec.image to the CI, keeping a registry set on the
// CI directly, and fills in the defaults.
func (r *ServiceReconciler) applyRegistry(svc *koptanv1.Service, ci *koptanv1.CI) {
	if img := svc.Spec.Image; img != nil {
		ci.Spec.Registry.Registry = img.Registry
		ci.Spec.Registry.Repo = img.Repo
		ci.Spec.Registry.CredentialsSecret = img.CredentialsSecret
	}
	if ci.Spec.Registry.Registry == "" {
		ci.Spec.Registry.Registry = r.DefaultRegistry
	}
	if ci.Spec.Registry.Registry == "" {
		ci.Spec.Registry.Registry = "docker.io"
	}
	if ci.Spec.Registry.Repo == "" {
		ci.Spec.Registry.Repo = svc.Name
	}
}
