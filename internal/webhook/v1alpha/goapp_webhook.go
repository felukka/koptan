package v1alpha

import (
	"context"
	"regexp"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	koptanv1alpha "github.com/felukka/koptan/api/v1alpha"
)

// nolint:unused
// log is for logging in this package.
var goapplog = logf.Log.WithName("goapp-resource")

// SetupGoAppWebhookWithManager registers the webhook for GoApp in the manager.
func SetupGoAppWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&koptanv1alpha.GoApp{}).
		WithValidator(&GoAppCustomValidator{}).
		WithDefaulter(&GoAppCustomDefaulter{}).
		Complete()
}

type GoAppCustomDefaulter struct {
	// TODO(user): Add more fields as needed for defaulting
}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the Kind GoApp.
func (d *GoAppCustomDefaulter) Default(ctx context.Context, obj runtime.Object) error {
	// Type assertion to ensure that obj is a *GoApp
	goApp, ok := obj.(*koptanv1alpha.GoApp)
	if !ok {
		return apierrors.NewBadRequest("expected GoApp object")
	}

	// Log the defaulting action
	goapplog.Info("Defaulting for GoApp", "name", goApp.GetName())

	// Default for GoVersion
	if goApp.Spec.GoVersion == "" {
		goApp.Spec.GoVersion = "1.26.1"
	}

	// Default for Entrypoint
	if goApp.Spec.Entrypoint == "" {
		goApp.Spec.Entrypoint = "main.go"
	}

	// Default for BuildArgs
	if goApp.Spec.BuildArgs == nil {
		goApp.Spec.BuildArgs = []string{} // No default build arguments
	}

	// Default for ExtraPackages
	if goApp.Spec.ExtraPackages == nil {
		goApp.Spec.ExtraPackages = []string{} // No default packages
	}

	// Default for Env
	if goApp.Spec.Env == nil {
		goApp.Spec.Env = map[string]string{} // Empty environment if not set
	}

	return nil
}

type GoAppCustomValidator struct{}

func (v *GoAppCustomValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	// Type assertion to ensure that obj is a *GoApp
	goApp, ok := obj.(*koptanv1alpha.GoApp)
	if !ok {
		return nil, apierrors.NewBadRequest("expected GoApp object")
	}

	// Log the validation action
	goapplog.Info("Validation for GoApp upon creation", "name", goApp.GetName())

	// Call the validation logic for GoApp
	return nil, v.validateGoApp(goApp)
}

func (v *GoAppCustomValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	// Type assertion to ensure that newObj is a *GoApp
	newGoApp, ok := newObj.(*koptanv1alpha.GoApp)
	if !ok {
		return nil, apierrors.NewBadRequest("expected GoApp object")
	}

	// Log the validation action
	goapplog.Info("Validation for GoApp upon update", "name", newGoApp.GetName())

	// Call the same validation logic for updates
	return nil, v.validateGoApp(newGoApp)
}

func (v *GoAppCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	// Type assertion to ensure that obj is a *GoApp
	goApp, ok := obj.(*koptanv1alpha.GoApp)
	if !ok {
		return nil, apierrors.NewBadRequest("expected GoApp object")
	}

	// Log the validation action
	goapplog.Info("Validation for GoApp upon deletion", "name", goApp.GetName())

	// No specific validation required for delete
	return nil, nil
}

func (v *GoAppCustomValidator) validateGoApp(obj *koptanv1alpha.GoApp) error {
	var allErrs field.ErrorList
	specPath := field.NewPath("spec")

	// Validate Source URL
	sourcePath := specPath.Child("source")
	if obj.Spec.Source.Repo == "" {
		allErrs = append(allErrs, field.Required(sourcePath.Child("repo"), "source repo is required"))
	} else {
		if !strings.HasPrefix(obj.Spec.Source.Repo, "https://") {
			allErrs = append(allErrs, field.Invalid(sourcePath.Child("repo"), obj.Spec.Source.Repo, "URL must use https:// protocol"))
		}
		if !strings.Contains(obj.Spec.Source.Repo, "@") {
			allErrs = append(allErrs, field.Invalid(sourcePath.Child("repo"), obj.Spec.Source.Repo, "URL must contain an '@' symbol"))
		}
	}

	version := strings.TrimSpace(obj.Spec.GoVersion)
	var versionRegex = regexp.MustCompile(`^\d+(\.\d+)*$`)

	if !versionRegex.MatchString(version) {
		allErrs = append(allErrs, field.Invalid(
			specPath.Child("goVersion"),
			obj.Spec.GoVersion,
			"Invalid Go version format. Must be a numeric version (e.g., '1.2.3' or '5.0')",
		))
	}

	if obj.Spec.Env != nil {
		envPath := specPath.Child("env")
		for key := range obj.Spec.Env {
			if key == "" {
				allErrs = append(allErrs, field.Invalid(envPath, key, "environment variable keys cannot be empty"))
			}
		}
	}

	// If any validation errors exist, return them
	if len(allErrs) == 0 {
		return nil
	}

	return apierrors.NewInvalid(
		schema.GroupKind{Group: "koptan.felukka.sh", Kind: "GoApp"},
		obj.Name,
		allErrs,
	)
}
