package v1alpha

import (
	"context"
<<<<<<< HEAD

=======
	"regexp"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
>>>>>>> tmp-original-31-03-26-02-51
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	koptanv1alpha "github.com/felukka/koptan/api/v1alpha"
)

const (
	BuildToolMaven  = "maven"
	BuildToolGradle = "gradle"
)

// nolint:unused
// log is for logging in this package.
var javaapplog = logf.Log.WithName("javaapp-resource")

// SetupJavaAppWebhookWithManager registers the webhook for JavaApp in the manager.
func SetupJavaAppWebhookWithManager(mgr ctrl.Manager) error {
<<<<<<< HEAD
	return ctrl.NewWebhookManagedBy(mgr, &koptanv1alpha.JavaApp{}).
=======
	return ctrl.NewWebhookManagedBy(mgr).
		For(&koptanv1alpha.JavaApp{}).
>>>>>>> tmp-original-31-03-26-02-51
		WithValidator(&JavaAppCustomValidator{}).
		WithDefaulter(&JavaAppCustomDefaulter{}).
		Complete()
}

type JavaAppCustomDefaulter struct {
	// TODO(user): Add more fields as needed for defaulting
}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the Kind JavaApp.
<<<<<<< HEAD
func (d *JavaAppCustomDefaulter) Default(_ context.Context, obj *koptanv1alpha.JavaApp) error {
	javaapplog.Info("Defaulting for JavaApp", "name", obj.GetName())
=======
func (d *JavaAppCustomDefaulter) Default(ctx context.Context, obj runtime.Object) error {
	javaApp, ok := obj.(*koptanv1alpha.JavaApp)
	if !ok {
		return apierrors.NewBadRequest("expected JavaApp object")
	}
	javaapplog.Info("Defaulting for JavaApp", "name", javaApp.GetName())
>>>>>>> tmp-original-31-03-26-02-51

	if javaApp.Spec.BuildTool == BuildToolMaven && javaApp.Spec.MavenGoal == "" {
		javaApp.Spec.MavenGoal = "package"
	}

	if javaApp.Spec.BuildArgs == nil {
		javaApp.Spec.BuildArgs = []string{}
	}

	if javaApp.Spec.ExtraPackages == nil {
		javaApp.Spec.ExtraPackages = []string{}
	}

	if javaApp.Spec.Env == nil {
		javaApp.Spec.Env = map[string]string{}
	}

	return nil
}

type JavaAppCustomValidator struct {
	// TODO(user): Add more fields as needed for validation
}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type JavaApp.
<<<<<<< HEAD
func (v *JavaAppCustomValidator) ValidateCreate(_ context.Context, obj *koptanv1alpha.JavaApp) (admission.Warnings, error) {
	javaapplog.Info("Validation for JavaApp upon creation", "name", obj.GetName())

	// TODO(user): fill in your validation logic upon object creation.

	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type JavaApp.
func (v *JavaAppCustomValidator) ValidateUpdate(_ context.Context, oldObj, newObj *koptanv1alpha.JavaApp) (admission.Warnings, error) {
	javaapplog.Info("Validation for JavaApp upon update", "name", newObj.GetName())

	// TODO(user): fill in your validation logic upon object update.

	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type JavaApp.
func (v *JavaAppCustomValidator) ValidateDelete(_ context.Context, obj *koptanv1alpha.JavaApp) (admission.Warnings, error) {
	javaapplog.Info("Validation for JavaApp upon deletion", "name", obj.GetName())

	// TODO(user): fill in your validation logic upon object deletion.

=======
func (v *JavaAppCustomValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	javaApp, ok := obj.(*koptanv1alpha.JavaApp)
	if !ok {
		return nil, apierrors.NewBadRequest("expected JavaApp object")
	}
	javaapplog.Info("Validation for JavaApp upon creation", "name", javaApp.GetName())
	return nil, v.validateJavaApp(javaApp)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type JavaApp.
func (v *JavaAppCustomValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	newJavaApp, ok := newObj.(*koptanv1alpha.JavaApp)
	if !ok {
		return nil, apierrors.NewBadRequest("expected JavaApp object")
	}
	javaapplog.Info("Validation for JavaApp upon update", "name", newJavaApp.GetName())
	return nil, v.validateJavaApp(newJavaApp)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type JavaApp.
func (v *JavaAppCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	javaApp, ok := obj.(*koptanv1alpha.JavaApp)
	if !ok {
		return nil, apierrors.NewBadRequest("expected JavaApp object")
	}
	javaapplog.Info("Validation for JavaApp upon deletion", "name", javaApp.GetName())
>>>>>>> tmp-original-31-03-26-02-51
	return nil, nil
}

func (v *JavaAppCustomValidator) validateJavaApp(obj *koptanv1alpha.JavaApp) error {
	var allErrs field.ErrorList
	specPath := field.NewPath("spec")

	sourcePath := specPath.Child("source")
	if obj.Spec.Source.Repo == "" {
		allErrs = append(allErrs, field.Required(sourcePath.Child("repo"), "source repo is required"))
	} else {
		if !strings.HasPrefix(obj.Spec.Source.Repo, "https://") {
			allErrs = append(allErrs, field.Invalid(sourcePath.Child("repo"), obj.Spec.Source.Repo, "URL must use https:// protocol"))
		}
	}

	version := strings.TrimSpace(obj.Spec.JavaVersion)
	versionRegex := regexp.MustCompile(`^\d+(\.\d+)*$`)

	if !versionRegex.MatchString(version) {
		allErrs = append(allErrs, field.Invalid(
			specPath.Child("javaVersion"),
			obj.Spec.JavaVersion,
			"Invalid Java version format (e.g., '17', '11', '1.8')",
		))
	}

	if obj.Spec.BuildTool != "" &&
		obj.Spec.BuildTool != BuildToolMaven &&
		obj.Spec.BuildTool != BuildToolGradle {

		allErrs = append(allErrs, field.Invalid(
			specPath.Child("buildTool"),
			obj.Spec.BuildTool,
			"must be either 'maven' or 'gradle'",
		))
	}

	if obj.Spec.ArtifactPath != "" {
		artifactPath := strings.TrimSpace(obj.Spec.ArtifactPath)
		if strings.HasPrefix(artifactPath, "/") {
			allErrs = append(allErrs, field.Invalid(
				specPath.Child("artifactPath"),
				obj.Spec.ArtifactPath,
				"artifactPath must be relative to the repository root and must not start with '/'",
			))
		} else {
			pathSegments := strings.Split(artifactPath, "/")
			for _, segment := range pathSegments {
				if segment == ".." {
					allErrs = append(allErrs, field.Invalid(
						specPath.Child("artifactPath"),
						obj.Spec.ArtifactPath,
						"artifactPath must not contain '..' path traversal",
					))
					break
				}
			}
		}
	}

	if obj.Spec.BuildTool == BuildToolMaven {
		if obj.Spec.GradleTask != "" && obj.Spec.GradleTask != "build" {
			allErrs = append(allErrs, field.Invalid(
				specPath.Child("gradleTask"),
				obj.Spec.GradleTask,
				"gradleTask must be empty or 'build' when using maven",
			))
		}
	}

	if obj.Spec.BuildTool == BuildToolGradle {
		if obj.Spec.MavenGoal != "" && obj.Spec.MavenGoal != "package" {
			allErrs = append(allErrs, field.Invalid(
				specPath.Child("mavenGoal"),
				obj.Spec.MavenGoal,
				"mavenGoal must be empty or \"package\" when using gradle",
			))
		}
	}

	if obj.Spec.Env != nil {
		envPath := specPath.Child("env")
		for key := range obj.Spec.Env {
			if key == "" {
				allErrs = append(allErrs, field.Invalid(envPath, key, "environment variable keys cannot be empty"))
			}
		}
	}

	if len(allErrs) == 0 {
		return nil
	}

	return apierrors.NewInvalid(
		schema.GroupKind{Group: "koptan.felukka.sh", Kind: "JavaApp"},
		obj.Name,
		allErrs,
	)
}
