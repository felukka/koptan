package controller

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

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
	"github.com/felukka/koptan/internal/appfactory"
)

const (
	appFinalizer   = "felukka.org/app-cleanup"
	dockerfileKey  = "dockerfile"
	patSecretKey   = "token"
	authSecretName = "-git-auth"
)

type AppReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=koptan.felukka.org,resources=goapps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=koptan.felukka.org,resources=goapps/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=koptan.felukka.org,resources=goapps/finalizers,verbs=update
// +kubebuilder:rbac:groups=koptan.felukka.org,resources=dotnetapps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=koptan.felukka.org,resources=dotnetapps/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=koptan.felukka.org,resources=dotnetapps/finalizers,verbs=update
// +kubebuilder:rbac:groups=koptan.felukka.org,resources=javaapps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=koptan.felukka.org,resources=javaapps/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=koptan.felukka.org,resources=javaapps/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch

func (r *AppReconciler) ReconcileGoApp(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var goApp koptan.GoApp
	if err := r.Get(ctx, req.NamespacedName, &goApp); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	return r.reconcile(ctx, &goAppAdapter{&goApp})
}

func (r *AppReconciler) ReconcileDotnetApp(
	ctx context.Context,
	req ctrl.Request,
) (ctrl.Result, error) {
	var dotnetApp koptan.DotnetApp
	if err := r.Get(ctx, req.NamespacedName, &dotnetApp); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	return r.reconcile(ctx, &dotnetAppAdapter{&dotnetApp})
}

func (r *AppReconciler) ReconcileJavaApp(
	ctx context.Context,
	req ctrl.Request,
) (ctrl.Result, error) {
	var javaApp koptan.JavaApp
	if err := r.Get(ctx, req.NamespacedName, &javaApp); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	return r.reconcile(ctx, &javaAppAdapter{&javaApp})
}

func (r *AppReconciler) reconcile(ctx context.Context, app App) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	if !app.GetDeletionTimestamp().IsZero() {
		if controllerutil.ContainsFinalizer(app, appFinalizer) {
			controllerutil.RemoveFinalizer(app, appFinalizer)
			if err := r.Update(ctx, app.RuntimeObject()); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(app, appFinalizer) {
		controllerutil.AddFinalizer(app, appFinalizer)
		if err := r.Update(ctx, app.RuntimeObject()); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	if app.GetAppPhase() == koptan.AppPhaseReady &&
		app.GetObservedGeneration() >= app.GetGeneration() {
		return ctrl.Result{}, nil
	}

	if app.GetAppPhase() != koptan.AppPhaseDiscovering {
		app.SetAppPhase(koptan.AppPhaseDiscovering)
		if err := r.statusUpdate(ctx, app); err != nil {
			return ctrl.Result{}, err
		}
	}

	src := app.GetSourceRef()

	token, err := r.resolveToken(ctx, app.GetNamespace(), src)
	if err != nil {
		return ctrl.Result{}, r.failWith(ctx, app, "TokenResolveFailed", err.Error())
	}

	if token != "" {
		if err := r.ensureAuthSecret(ctx, app, token); err != nil {
			return ctrl.Result{}, r.failWith(ctx, app, "AuthSecretFailed", err.Error())
		}
	}

	cloneDir, err := os.MkdirTemp("", "koptan-discover-*")
	if err != nil {
		return ctrl.Result{}, r.failWith(ctx, app, "TmpDirFailed", err.Error())
	}
	defer func() { _ = os.RemoveAll(cloneDir) }()

	log.Info("cloning source", "repo", src.Repo, "revision", src.Revision)
	_, err = appfactory.Checkout(appfactory.CloneOptions{
		Repo:     src.Repo,
		Revision: src.Revision,
		Token:    token,
		Dir:      cloneDir,
	})
	if err != nil {
		return ctrl.Result{}, r.failWith(
			ctx,
			app,
			"CloneFailed",
			fmt.Sprintf("clone failed: %v", err),
		)
	}

	dockerfileName := "Dockerfile" // The default Dockerfile
	// check if a different dockerfile name is given
	if src.DockerfileName != "" {
		dockerfileName = src.DockerfileName
	}

	dockerfilePath := filepath.Join(cloneDir, dockerfileName)
	var content []byte

	// Check if the Dockerfile exists at the specified path
	if _, err := os.Stat(dockerfilePath); err == nil {
		log.Info("Dockerfile found in repo, using existing Dockerfile", "path", dockerfilePath)
		content, err = os.ReadFile(dockerfilePath)
		if err != nil {
			log.Error(err, "Failed to read Dockerfile")
			return ctrl.Result{}, err
		}
	} else {
		log.Info("No Dockerfile found, running discovery", "repo", src.Repo)
		contentStr, err := app.RunDiscoveryAndGenerate(cloneDir)
		if err != nil {
			return ctrl.Result{}, r.failWith(ctx, app, "DiscoveryFailed", err.Error())
		}
		content = []byte(contentStr)
	}

	cmName := app.GetName() + "-dockerfile"
	if err := r.reconcileConfigMap(ctx, app, cmName, string(content)); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.setReady(ctx, app, cmName); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *AppReconciler) resolveToken(
	ctx context.Context,
	namespace string,
	src koptan.SourceRef,
) (string, error) {
	if src.PATToken == "" {
		return "", nil
	}

	var secret corev1.Secret
	key := types.NamespacedName{Name: src.PATToken, Namespace: namespace}
	if err := r.Get(ctx, key, &secret); err != nil {
		if errors.IsNotFound(err) {
			return "", fmt.Errorf(
				"pat secret %q not found in namespace %q",
				src.PATToken,
				namespace,
			)
		}
		return "", fmt.Errorf("getting pat secret %q: %w", src.PATToken, err)
	}

	tokenBytes, ok := secret.Data[patSecretKey]
	if !ok {
		return "", fmt.Errorf("pat secret %q has no %q key", src.PATToken, patSecretKey)
	}
	return string(tokenBytes), nil
}

func (r *AppReconciler) ensureAuthSecret(ctx context.Context, app App, token string) error {
	secretName := app.GetName() + authSecretName

	desired := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: app.GetNamespace(),
			Labels: map[string]string{
				"felukka.org/app":       app.GetName(),
				"felukka.org/component": "git-auth",
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			patSecretKey: []byte(token),
		},
	}

	if err := controllerutil.SetOwnerReference(app.RuntimeObject(), desired, r.Scheme); err != nil {
		return fmt.Errorf("setting owner reference on auth secret: %w", err)
	}

	var existing corev1.Secret
	key := types.NamespacedName{Name: secretName, Namespace: app.GetNamespace()}
	err := r.Get(ctx, key, &existing)

	if errors.IsNotFound(err) {
		return r.Create(ctx, desired)
	}
	if err != nil {
		return fmt.Errorf("checking auth secret %q: %w", secretName, err)
	}

	if string(existing.Data[patSecretKey]) != token {
		existing.Data = desired.Data
		existing.Labels = desired.Labels
		return r.Update(ctx, &existing)
	}
	return nil
}

func AuthSecretNameFor(appName string) string {
	return appName + authSecretName
}

func (r *AppReconciler) reconcileConfigMap(
	ctx context.Context,
	app App,
	cmName, content string,
) error {
	log := logf.FromContext(ctx)

	desired := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmName,
			Namespace: app.GetNamespace(),
			Labels: map[string]string{
				"felukka.org/app":       app.GetName(),
				"felukka.org/app-kind":  app.GetObjectKind().GroupVersionKind().Kind,
				"felukka.org/component": "dockerfile",
			},
		},
		Data: map[string]string{
			dockerfileKey: content,
		},
	}

	if err := controllerutil.SetControllerReference(app.RuntimeObject(), desired, r.Scheme); err != nil {
		return fmt.Errorf("setting controller reference on ConfigMap: %w", err)
	}

	var existing corev1.ConfigMap
	key := types.NamespacedName{Name: cmName, Namespace: app.GetNamespace()}
	err := r.Get(ctx, key, &existing)

	switch {
	case errors.IsNotFound(err):
		log.Info("creating Dockerfile ConfigMap", "configmap", cmName)
		return r.Create(ctx, desired)
	case err != nil:
		return fmt.Errorf("getting ConfigMap %s: %w", cmName, err)
	default:
		if existing.Data[dockerfileKey] != content {
			existing.Data = desired.Data
			existing.Labels = desired.Labels
			log.Info("updating Dockerfile ConfigMap", "configmap", cmName)
			return r.Update(ctx, &existing)
		}
	}
	return nil
}

func (r *AppReconciler) setReady(ctx context.Context, app App, cmName string) error {
	app.SetAppPhase(koptan.AppPhaseReady)
	app.SetObservedGeneration(app.GetGeneration())
	app.SetConfigMapName(cmName)
	app.SetError("")

	conditions := app.GetConditions()
	meta.SetStatusCondition(conditions, metav1.Condition{
		Type:               koptan.AppConditionDockerfileGenerated,
		Status:             metav1.ConditionTrue,
		Reason:             "Generated",
		Message:            "Dockerfile generated from discovered source",
		ObservedGeneration: app.GetGeneration(),
	})
	meta.SetStatusCondition(conditions, metav1.Condition{
		Type:               koptan.AppConditionConfigMapReady,
		Status:             metav1.ConditionTrue,
		Reason:             "ConfigMapReady",
		Message:            fmt.Sprintf("ConfigMap %s is up to date", cmName),
		ObservedGeneration: app.GetGeneration(),
	})

	return r.statusUpdate(ctx, app)
}

func (r *AppReconciler) failWith(ctx context.Context, app App, reason, msg string) error {
	if err := r.setFailed(ctx, app, reason, msg); err != nil {
		return err
	}

	return fmt.Errorf("%s: %s", reason, msg)
}

func (r *AppReconciler) setFailed(ctx context.Context, app App, reason, msg string) error {
	app.SetAppPhase(koptan.AppPhaseFailed)
	app.SetError(msg)

	conditions := app.GetConditions()
	meta.SetStatusCondition(conditions, metav1.Condition{
		Type:               koptan.AppConditionDockerfileGenerated,
		Status:             metav1.ConditionFalse,
		Reason:             reason,
		Message:            msg,
		ObservedGeneration: app.GetGeneration(),
	})

	return r.statusUpdate(ctx, app)
}

func (r *AppReconciler) statusUpdate(ctx context.Context, app App) error {
	return r.Status().Update(ctx, app.RuntimeObject())
}

func (r *AppReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := ctrl.NewControllerManagedBy(mgr).
		For(&koptan.GoApp{}).
		Owns(&corev1.ConfigMap{}).
		Named("goapp").
		Complete(reconcilerFunc(r.ReconcileGoApp)); err != nil {
		return fmt.Errorf("setting up GoApp controller: %w", err)
	}

	if err := ctrl.NewControllerManagedBy(mgr).
		For(&koptan.DotnetApp{}).
		Owns(&corev1.ConfigMap{}).
		Named("dotnetapp").
		Complete(reconcilerFunc(r.ReconcileDotnetApp)); err != nil {
		return fmt.Errorf("setting up DotnetApp controller: %w", err)
	}

	if err := ctrl.NewControllerManagedBy(mgr).
		For(&koptan.JavaApp{}).
		Owns(&corev1.ConfigMap{}).
		Named("javaapp").
		Complete(reconcilerFunc(r.ReconcileJavaApp)); err != nil {
		return fmt.Errorf("setting up JavaApp controller: %w", err)
	}

	return nil
}

type reconcilerFunc func(context.Context, ctrl.Request) (ctrl.Result, error)

func (f reconcilerFunc) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return f(ctx, req)
}
