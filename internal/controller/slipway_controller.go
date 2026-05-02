package controller

import (
	"context"
	"fmt"
	"sort"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	koptan "github.com/felukka/koptan/api/v1alpha"
	"github.com/felukka/koptan/internal/utils"
)

const (
	slipwayFinalizer = "felukka.org/slipway-cleanup"
	condReady        = "Ready"
	condBuild        = "BuildSucceeded"

	gitImage     = "alpine/git:2.47.2"
	buildahImage = "quay.io/buildah/stable:v1.43.0"

	workspacePath      = "/tmp/workspace"
	dockerfilePath     = "/dockerfile"
	dockerfileVolume   = "app-dockerfile"
	dockerConfigVolume = "docker-config"
	dockerConfigPath   = "/auth"
)

type SlipwayReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=koptan.felukka.org,resources=slipways,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=koptan.felukka.org,resources=slipways/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=koptan.felukka.org,resources=slipways/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch
// +kubebuilder:rbac:groups=koptan.felukka.org,resources=goapps,verbs=get;list;watch
// +kubebuilder:rbac:groups=koptan.felukka.org,resources=goapps/status,verbs=get
// +kubebuilder:rbac:groups=koptan.felukka.org,resources=dotnetapps,verbs=get;list;watch
// +kubebuilder:rbac:groups=koptan.felukka.org,resources=dotnetapps/status,verbs=get
// +kubebuilder:rbac:groups=koptan.felukka.org,resources=javaapps,verbs=get;list;watch
// +kubebuilder:rbac:groups=koptan.felukka.org,resources=javaapps/status,verbs=get

func (r *SlipwayReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var sw koptan.Slipway
	if err := r.Get(ctx, req.NamespacedName, &sw); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !sw.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(&sw, slipwayFinalizer) {
			controllerutil.RemoveFinalizer(&sw, slipwayFinalizer)
			return ctrl.Result{}, r.Update(ctx, &sw)
		}
		return ctrl.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(&sw, slipwayFinalizer) {
		controllerutil.AddFinalizer(&sw, slipwayFinalizer)
		if err := r.Update(ctx, &sw); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	if sw.Status.Phase == koptan.SlipwayPhaseBuilding ||
		sw.Status.Phase == koptan.SlipwayPhaseResolving {
		return r.trackBuild(ctx, &sw)
	}

	appPhase, configMapName, sourceRef, err := r.resolveApp(ctx, &sw)
	if err != nil {
		log.Error(err, "failed to resolve app")
		_ = r.updateStatus(ctx, req.NamespacedName, func(s *koptan.Slipway) {
			s.Status.Phase = koptan.SlipwayPhaseFailed
			s.Status.Message = fmt.Sprintf("app resolution failed: %v", err)
		})
		return ctrl.Result{}, err
	}

	if appPhase != koptan.AppPhaseReady {
		_ = r.updateStatus(ctx, req.NamespacedName, func(s *koptan.Slipway) {
			s.Status.Phase = koptan.SlipwayPhaseIdle
			s.Status.Message = fmt.Sprintf("waiting for app to be Ready (current: %s)", appPhase)
		})
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	patToken := r.resolveGitToken(ctx, sw.Namespace, sw.Spec.AppRef.Name, sourceRef)

	revision, err := utils.ResolveBranch(ctx, sourceRef.Repo, sourceRef.Revision, patToken)
	if err != nil {
		log.Error(err, "failed to resolve revision")
		_ = r.updateStatus(ctx, req.NamespacedName, func(s *koptan.Slipway) {
			s.Status.Phase = koptan.SlipwayPhaseFailed
			s.Status.Message = fmt.Sprintf("git resolve failed: %v", err)
		})
		return ctrl.Result{}, err
	}

	if revision.SHA == sw.Status.LatestRevision &&
		(sw.Status.Phase == koptan.SlipwayPhaseSucceeded || sw.Status.Phase == koptan.SlipwayPhaseFailed) {
		return ctrl.Result{}, nil
	}

	log.Info("new revision detected", "sha", revision.SHA)

	dockerCfgSecret, err := r.ensureDockerConfigSecret(ctx, &sw)
	if err != nil {
		_ = r.updateStatus(ctx, req.NamespacedName, func(s *koptan.Slipway) {
			s.Status.Phase = koptan.SlipwayPhaseFailed
			s.Status.Message = fmt.Sprintf("docker config secret failed: %v", err)
		})
		return ctrl.Result{}, err
	}

	return r.startBuild(ctx, &sw, revision.SHA, configMapName, sourceRef, dockerCfgSecret)
}

func (r *SlipwayReconciler) resolveGitToken(
	ctx context.Context,
	namespace, appName string,
	sourceRef koptan.SourceRef,
) string {
	if sourceRef.PATToken == "" {
		return ""
	}
	authSecretName := AuthSecretNameFor(appName)
	var secret corev1.Secret
	if err := r.Get(ctx, types.NamespacedName{Name: authSecretName, Namespace: namespace}, &secret); err == nil {
		return string(secret.Data["token"])
	}
	var fallback corev1.Secret
	if err := r.Get(ctx, types.NamespacedName{Name: sourceRef.PATToken, Namespace: namespace}, &fallback); err == nil {
		return string(fallback.Data["token"])
	}
	return ""
}

func (r *SlipwayReconciler) resolveApp(
	ctx context.Context,
	sw *koptan.Slipway,
) (koptan.AppPhase, string, koptan.SourceRef, error) {
	ref := sw.Spec.AppRef
	ns := sw.Namespace
	switch ref.Kind {
	case "GoApp":
		var app koptan.GoApp
		if err := r.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: ns}, &app); err != nil {
			return "", "", koptan.SourceRef{}, fmt.Errorf(
				"failed to get %s %q in namespace %q: %w",
				ref.Kind,
				ref.Name,
				ns,
				err,
			)
		}
		return app.Status.Phase, app.Status.ConfigMapName, app.Spec.Source, nil
	case "DotnetApp":
		var app koptan.DotnetApp
		if err := r.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: ns}, &app); err != nil {
			return "", "", koptan.SourceRef{}, fmt.Errorf(
				"failed to get %s %q in namespace %q: %w",
				ref.Kind,
				ref.Name,
				ns,
				err,
			)
		}
		return app.Status.Phase, app.Status.ConfigMapName, app.Spec.Source, nil
	case "JavaApp":
		var app koptan.JavaApp
		if err := r.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: ns}, &app); err != nil {
			return "", "", koptan.SourceRef{}, fmt.Errorf(
				"failed to get %s %q in namespace %q: %w",
				ref.Kind,
				ref.Name,
				ns,
				err,
			)
		}
		return app.Status.Phase, app.Status.ConfigMapName, app.Spec.Source, nil
	default:
		return "", "", koptan.SourceRef{}, fmt.Errorf("unsupported app kind %q", ref.Kind)
	}
}

func (r *SlipwayReconciler) ensureDockerConfigSecret(
	ctx context.Context,
	sw *koptan.Slipway,
) (string, error) {
	creds := sw.Spec.Image.Creds
	if creds == nil {
		return "", nil
	}
	secretName := sw.Name + "-registry-auth"
	configJSON, err := buildDockerConfigJSON(sw.Spec.Image.Registry, creds.Username, creds.Password)
	if err != nil {
		return "", err
	}
	desired := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: sw.Namespace,
			Labels:    map[string]string{"felukka.org/slipway": sw.Name},
		},
		Type: corev1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{".dockerconfigjson": configJSON},
	}
	if err := ctrl.SetControllerReference(sw, desired, r.Scheme); err != nil {
		return "", err
	}
	var existing corev1.Secret
	err = r.Get(ctx, types.NamespacedName{Name: secretName, Namespace: sw.Namespace}, &existing)
	if errors.IsNotFound(err) {
		if err := r.Create(ctx, desired); err != nil {
			return "", err
		}
		return secretName, nil
	}
	if err != nil {
		return "", err
	}
	existing.Data = desired.Data
	existing.Labels = desired.Labels
	if err := r.Update(ctx, &existing); err != nil {
		return "", err
	}
	return secretName, nil
}

func (r *SlipwayReconciler) hasActivePod(ctx context.Context, sw *koptan.Slipway) (bool, error) {
	var podList corev1.PodList

	if err := r.List(
		ctx,
		&podList,
		client.InNamespace(sw.Namespace),
		client.MatchingLabels{"felukka.org/slipway": sw.Name},
	); err != nil {
		return false, err
	}

	for _, pod := range podList.Items {
		phase := pod.Status.Phase
		if phase != corev1.PodSucceeded && phase != corev1.PodFailed {
			return true, nil
		}
	}

	return false, nil
}

func (r *SlipwayReconciler) startBuild(
	ctx context.Context,
	sw *koptan.Slipway,
	sha, configMapName string,
	sourceRef koptan.SourceRef,
	dockerCfgSecret string,
) (ctrl.Result, error) {
	active, err := r.hasActivePod(ctx, sw)
	if err != nil {
		return ctrl.Result{}, err
	}
	if active {
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	imageTag := fmt.Sprintf("%s/%s:%s", sw.Spec.Image.Registry, sw.Spec.Image.Name, sha[:12])
	pod := r.buildPod(sw, sha, configMapName, sourceRef, imageTag, dockerCfgSecret)
	if err := ctrl.SetControllerReference(sw, pod, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.Create(ctx, pod); err != nil {
		return ctrl.Result{}, err
	}

	err = r.updateStatus(
		ctx,
		types.NamespacedName{Name: sw.Name, Namespace: sw.Namespace},
		func(s *koptan.Slipway) {
			if s.Status.LatestRevision != sha {
				now := metav1.Now()
				s.Status.LastBuildTime = &now
				s.Status.BuildCount++
			}
			s.Status.Phase = koptan.SlipwayPhaseResolving
			s.Status.LatestRevision = sha
			s.Status.LatestImage = imageTag
			s.Status.Message = "build pod created"

			meta.SetStatusCondition(&s.Status.Conditions, metav1.Condition{
				Type: condBuild, Status: metav1.ConditionUnknown, Reason: "BuildStarted", Message: "build pod created",
			})
			meta.SetStatusCondition(&s.Status.Conditions, metav1.Condition{
				Type: condReady, Status: metav1.ConditionFalse, Reason: "Building", Message: "build in progress",
			})
		},
	)

	return ctrl.Result{RequeueAfter: 10 * time.Second}, err
}

func (r *SlipwayReconciler) trackBuild(
	ctx context.Context,
	sw *koptan.Slipway,
) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)
	var podList corev1.PodList
	if err := r.List(ctx, &podList, client.InNamespace(sw.Namespace), client.MatchingLabels{"felukka.org/slipway": sw.Name}); err != nil {
		return ctrl.Result{}, err
	}

	if len(podList.Items) == 0 {
		err := r.updateStatus(
			ctx,
			types.NamespacedName{Name: sw.Name, Namespace: sw.Namespace},
			func(s *koptan.Slipway) {
				s.Status.Phase = koptan.SlipwayPhaseFailed
				s.Status.Message = "build pod disappeared"
			},
		)
		if err != nil {
			logger.Error(err, "failed to update Slipway status when build pod disappeared",
				"slipway", sw.Name, "namespace", sw.Namespace)
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	}

	sort.Slice(podList.Items, func(i, j int) bool {
		return podList.Items[i].CreationTimestamp.Before(&podList.Items[j].CreationTimestamp)
	})
	pod := &podList.Items[len(podList.Items)-1]

	phase, msg, finished := simplifyPodStatus(pod)

	if finished {
		if isBuildSucceeded(phase) {
			if delErr := r.Delete(ctx, pod); delErr != nil && !errors.IsNotFound(delErr) {
				return ctrl.Result{}, delErr
			}
			logger.Info("Successfully built, pod deleted", "pod", pod.Name)
		}

		err := r.updateStatus(
			ctx,
			types.NamespacedName{Name: sw.Name, Namespace: sw.Namespace},
			func(s *koptan.Slipway) {
				if phase == koptan.SlipwayPhaseSucceeded {
					s.Status.Phase = koptan.SlipwayPhaseSucceeded
					s.Status.Message = "build succeeded"
					meta.SetStatusCondition(&s.Status.Conditions, metav1.Condition{
						Type: condBuild, Status: metav1.ConditionTrue, Reason: "BuildSucceeded",
					})
					meta.SetStatusCondition(&s.Status.Conditions, metav1.Condition{
						Type: condReady, Status: metav1.ConditionTrue, Reason: "ImageAvailable",
					})
				} else {
					s.Status.Phase = koptan.SlipwayPhaseFailed
					s.Status.Message = msg
					meta.SetStatusCondition(&s.Status.Conditions, metav1.Condition{
						Type: condBuild, Status: metav1.ConditionFalse, Reason: "BuildFailed", Message: msg,
					})
				}
			},
		)
		if err != nil {
			return ctrl.Result{}, err
		}
		//if delErr := r.Delete(ctx, pod); delErr != nil && !errors.IsNotFound(delErr) {
		//	return ctrl.Result{}, delErr
		//}
		return ctrl.Result{}, nil
	}

	if sw.Status.Message != msg {
		err := r.updateStatus(
			ctx,
			types.NamespacedName{Name: sw.Name, Namespace: sw.Namespace},
			func(s *koptan.Slipway) {
				s.Status.Phase = koptan.SlipwayPhaseBuilding
				s.Status.Message = msg
			},
		)
		if err != nil {
			logger.Error(err, "failed to update Slipway status during build",
				"slipway", sw.Name, "namespace", sw.Namespace)
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
}

func isBuildSucceeded(phase koptan.SlipwayPhase) bool {
	return phase == koptan.SlipwayPhaseSucceeded
}

func simplifyPodStatus(pod *corev1.Pod) (koptan.SlipwayPhase, string, bool) {
	if pod.Status.Phase == corev1.PodSucceeded {
		return koptan.SlipwayPhaseSucceeded, "build completed", true
	}
	if pod.Status.Phase == corev1.PodFailed {
		msg := "pod failed"
		if pod.Status.Message != "" {
			msg = pod.Status.Message
		}
		for _, c := range append(pod.Status.InitContainerStatuses, pod.Status.ContainerStatuses...) {
			if t := c.State.Terminated; t != nil && t.ExitCode != 0 {
				detail := t.Reason
				if detail == "" {
					detail = t.Message
				}
				if detail != "" {
					msg = fmt.Sprintf(
						"container %s failed with exit code %d: %s",
						c.Name,
						t.ExitCode,
						detail,
					)
				} else {
					msg = fmt.Sprintf("container %s failed with exit code %d", c.Name, t.ExitCode)
				}
				return koptan.SlipwayPhaseFailed, msg, true
			}
		}
		return koptan.SlipwayPhaseFailed, msg, true
	}

	msg := "pod pending"
	if pod.Status.Phase == corev1.PodRunning {
		msg = "pod running"
	}
	for _, c := range pod.Status.ContainerStatuses {
		if c.State.Waiting != nil {
			msg = fmt.Sprintf("%s: %s", c.Name, c.State.Waiting.Reason)
		}
	}
	return koptan.SlipwayPhaseBuilding, msg, false
}

func (r *SlipwayReconciler) updateStatus(
	ctx context.Context,
	name types.NamespacedName,
	modify func(*koptan.Slipway),
) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var sw koptan.Slipway
		if err := r.Get(ctx, name, &sw); err != nil {
			return err
		}
		modify(&sw)
		return r.Status().Update(ctx, &sw)
	})
}

func (r *SlipwayReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&koptan.Slipway{}).
		Owns(&corev1.Pod{}).
		Named("slipway").
		Complete(r)
}
