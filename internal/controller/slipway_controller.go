package controller

import (
	"context"
	"fmt"
	"sort"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	koptan "github.com/felukka/koptan/api/v1alpha"
	"github.com/felukka/koptan/internal/utils"
)

const (
	slipwayFinalizer = "felukka.sh/slipway-cleanup"
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

// +kubebuilder:rbac:groups=koptan.felukka.sh,resources=slipways,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=koptan.felukka.sh,resources=slipways/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=koptan.felukka.sh,resources=slipways/finalizers,verbs=update
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;delete
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch
// +kubebuilder:rbac:groups=koptan.felukka.sh,resources=goapps,verbs=get;list;watch
// +kubebuilder:rbac:groups=koptan.felukka.sh,resources=goapps/status,verbs=get
// +kubebuilder:rbac:groups=koptan.felukka.sh,resources=dotnetapps,verbs=get;list;watch
// +kubebuilder:rbac:groups=koptan.felukka.sh,resources=dotnetapps/status,verbs=get
// +kubebuilder:rbac:groups=koptan.felukka.sh,resources=javaapps,verbs=get;list;watch
// +kubebuilder:rbac:groups=koptan.felukka.sh,resources=javaapps/status,verbs=get

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
		err = r.setStatus(
			ctx,
			&sw,
			koptan.SlipwayPhaseFailed,
			fmt.Sprintf("app resolution failed: %v", err),
		)
		return ctrl.Result{RequeueAfter: 15 * time.Second}, err
	}

	if appPhase != koptan.AppPhaseReady {
		log.Info("app not ready yet", "phase", appPhase)
		err = r.setStatus(
			ctx,
			&sw,
			koptan.SlipwayPhaseIdle,
			fmt.Sprintf("waiting for app to be Ready (current: %s)", appPhase),
		)
		return ctrl.Result{RequeueAfter: 10 * time.Second}, err
	}

	if configMapName == "" {
		err = r.setStatus(ctx, &sw, koptan.SlipwayPhaseFailed, "app has no configMapName in status")
		return ctrl.Result{RequeueAfter: 15 * time.Second}, err
	}

	patToken := r.resolveGitToken(ctx, sw.Namespace, sw.Spec.AppRef.Name, sourceRef)

	revision, err := utils.ResolveBranch(ctx, sourceRef.Repo, sourceRef.Revision, patToken)
	if err != nil {
		log.Error(err, "failed to resolve revision")
		now := metav1.Now()
		sw.Status.LastPollTime = &now
		err = r.Status().Update(ctx, &sw)
		return r.requeuePoll(&sw), err
	}

	now := metav1.Now()
	sw.Status.LastPollTime = &now

	if revision.SHA == sw.Status.LatestRevision &&
		(sw.Status.Phase == koptan.SlipwayPhaseSucceeded || sw.Status.Phase == koptan.SlipwayPhaseFailed) {
		if err := r.Status().Update(ctx, &sw); err != nil {
			return ctrl.Result{}, err
		}
		return r.requeuePoll(&sw), nil
	}

	log.Info("new revision detected", "sha", revision.SHA)

	dockerCfgSecret, err := r.ensureDockerConfigSecret(ctx, &sw)
	if err != nil {
		log.Error(err, "failed to ensure docker config secret")
		err = r.setStatus(
			ctx,
			&sw,
			koptan.SlipwayPhaseFailed,
			fmt.Sprintf("docker config secret failed: %v", err),
		)
		return ctrl.Result{RequeueAfter: 15 * time.Second}, err
	}

	return r.startBuild(ctx, &sw, revision.SHA, configMapName, sourceRef, dockerCfgSecret)
}

func (r *SlipwayReconciler) resolveGitToken(
	ctx context.Context,
	namespace string,
	appName string,
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
			return "", "", koptan.SourceRef{}, fmt.Errorf("GoApp %q not found: %w", ref.Name, err)
		}
		return app.Status.Phase, app.Status.ConfigMapName, app.Spec.Source, nil

	case "DotnetApp":
		var app koptan.DotnetApp
		if err := r.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: ns}, &app); err != nil {
			return "", "", koptan.SourceRef{}, fmt.Errorf(
				"DotnetApp %q not found: %w",
				ref.Name,
				err,
			)
		}
		return app.Status.Phase, app.Status.ConfigMapName, app.Spec.Source, nil

	case "JavaApp":
		var app koptan.JavaApp
		if err := r.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: ns}, &app); err != nil {
			return "", "", koptan.SourceRef{}, fmt.Errorf("JavaApp %q not found: %w", ref.Name, err)
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
		return "", fmt.Errorf("building docker config json: %w", err)
	}

	desired := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: sw.Namespace,
			Labels: map[string]string{
				"felukka.sh/slipway":   sw.Name,
				"felukka.sh/component": "registry-auth",
			},
		},
		Type: corev1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{
			".dockerconfigjson": configJSON,
		},
	}

	if err := ctrl.SetControllerReference(sw, desired, r.Scheme); err != nil {
		return "", fmt.Errorf("setting owner reference on docker config secret: %w", err)
	}

	var existing corev1.Secret
	key := types.NamespacedName{Name: secretName, Namespace: sw.Namespace}
	err = r.Get(ctx, key, &existing)

	if errors.IsNotFound(err) {
		if err := r.Create(ctx, desired); err != nil {
			return "", fmt.Errorf("creating docker config secret: %w", err)
		}
		return secretName, nil
	}
	if err != nil {
		return "", fmt.Errorf("getting docker config secret: %w", err)
	}

	existing.Data = desired.Data
	existing.Labels = desired.Labels
	if err := r.Update(ctx, &existing); err != nil {
		return "", fmt.Errorf("updating docker config secret: %w", err)
	}
	return secretName, nil
}

func (r *SlipwayReconciler) hasActiveJob(
	ctx context.Context,
	sw *koptan.Slipway,
) (bool, error) {
	var jobList batchv1.JobList
	err := r.List(ctx, &jobList, client.InNamespace(sw.Namespace), client.MatchingLabels{
		"felukka.sh/slipway": sw.Name,
	})
	if err != nil {
		return false, err
	}
	for i := range jobList.Items {
		if jobList.Items[i].Status.Succeeded == 0 && jobList.Items[i].Status.Failed == 0 {
			return true, nil
		}
	}
	return false, nil
}

func (r *SlipwayReconciler) startBuild(
	ctx context.Context,
	sw *koptan.Slipway,
	sha string,
	configMapName string,
	sourceRef koptan.SourceRef,
	dockerCfgSecret string,
) (ctrl.Result, error) {
	active, err := r.hasActiveJob(ctx, sw)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("checking for active build jobs: %w", err)
	}
	if active {
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	imageTag := fmt.Sprintf("%s/%s:%s", sw.Spec.Image.Registry, sw.Spec.Image.Name, sha[:12])

	job := r.buildJob(sw, sha, configMapName, sourceRef, imageTag, dockerCfgSecret)
	if err := ctrl.SetControllerReference(sw, job, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.Create(ctx, job); err != nil {
		return ctrl.Result{}, fmt.Errorf("creating build job: %w", err)
	}

	key := types.NamespacedName{Name: sw.Name, Namespace: sw.Namespace}
	if err := r.Get(ctx, key, sw); err != nil {
		return ctrl.Result{}, fmt.Errorf("re-fetching slipway after job creation: %w", err)
	}

	if sw.Status.Phase == koptan.SlipwayPhaseResolving ||
		sw.Status.Phase == koptan.SlipwayPhaseBuilding {
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	now := metav1.Now()
	sw.Status.Phase = koptan.SlipwayPhaseResolving
	sw.Status.LatestRevision = sha
	sw.Status.LatestImage = imageTag
	sw.Status.LastBuildTime = &now
	sw.Status.BuildCount++
	sw.Status.Message = "build job created"

	meta.SetStatusCondition(&sw.Status.Conditions, metav1.Condition{
		Type:    condBuild,
		Status:  metav1.ConditionUnknown,
		Reason:  "BuildStarted",
		Message: fmt.Sprintf("build job created for revision %s", sha),
	})
	meta.SetStatusCondition(&sw.Status.Conditions, metav1.Condition{
		Type:    condReady,
		Status:  metav1.ConditionFalse,
		Reason:  "Building",
		Message: "build in progress",
	})

	if err := r.Status().Update(ctx, sw); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
}

func (r *SlipwayReconciler) trackBuild(
	ctx context.Context,
	sw *koptan.Slipway,
) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var jobList batchv1.JobList
	if err := r.List(ctx, &jobList, client.InNamespace(sw.Namespace), client.MatchingLabels{
		"felukka.sh/slipway": sw.Name,
	}); err != nil {
		return ctrl.Result{}, err
	}

	if len(jobList.Items) == 0 {
		log.Info("no build job found, resetting to idle")
		err := r.setStatus(ctx, sw, koptan.SlipwayPhaseFailed, "build job disappeared")
		return r.requeuePoll(sw), err
	}

	sort.Slice(jobList.Items, func(i, j int) bool {
		return jobList.Items[i].CreationTimestamp.Before(&jobList.Items[j].CreationTimestamp)
	})
	job := &jobList.Items[len(jobList.Items)-1]

	for _, c := range job.Status.Conditions {
		if c.Type == batchv1.JobComplete && c.Status == corev1.ConditionTrue {
			return r.buildSucceeded(ctx, sw)
		}
		if c.Type == batchv1.JobFailed && c.Status == corev1.ConditionTrue {
			return r.buildFailed(ctx, sw, c.Message)
		}
	}

	if sw.Status.Phase != koptan.SlipwayPhaseBuilding {
		sw.Status.Phase = koptan.SlipwayPhaseBuilding
		sw.Status.Message = "build running"
		if err := r.Status().Update(ctx, sw); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
}

func (r *SlipwayReconciler) buildSucceeded(
	ctx context.Context,
	sw *koptan.Slipway,
) (ctrl.Result, error) {
	sw.Status.Phase = koptan.SlipwayPhaseSucceeded
	sw.Status.Message = ""

	meta.SetStatusCondition(&sw.Status.Conditions, metav1.Condition{
		Type:   condBuild,
		Status: metav1.ConditionTrue,
		Reason: "BuildSucceeded",
	})
	meta.SetStatusCondition(&sw.Status.Conditions, metav1.Condition{
		Type:   condReady,
		Status: metav1.ConditionTrue,
		Reason: "ImageAvailable",
	})

	err := r.Status().Update(ctx, sw)
	return r.requeuePoll(sw), err
}

func (r *SlipwayReconciler) buildFailed(
	ctx context.Context,
	sw *koptan.Slipway,
	msg string,
) (ctrl.Result, error) {
	if msg == "" {
		msg = "build job failed"
	}

	meta.SetStatusCondition(&sw.Status.Conditions, metav1.Condition{
		Type:    condBuild,
		Status:  metav1.ConditionFalse,
		Reason:  "BuildFailed",
		Message: msg,
	})

	err := r.setStatus(ctx, sw, koptan.SlipwayPhaseFailed, msg)
	return r.requeuePoll(sw), err
}

func (r *SlipwayReconciler) setStatus(
	ctx context.Context,
	sw *koptan.Slipway,
	phase koptan.SlipwayPhase,
	msg string,
) error {
	sw.Status.Phase = phase
	sw.Status.Message = msg
	return r.Status().Update(ctx, sw)
}

func (r *SlipwayReconciler) requeuePoll(sw *koptan.Slipway) ctrl.Result {
	interval := sw.Spec.Poll.IntervalSeconds
	if interval <= 0 {
		interval = 60
	}
	return ctrl.Result{RequeueAfter: time.Duration(interval) * time.Second}
}

func (r *SlipwayReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&koptan.Slipway{}).
		Owns(&batchv1.Job{}).
		Named("slipway").
		Complete(r)
}
