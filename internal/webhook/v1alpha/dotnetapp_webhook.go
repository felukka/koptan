package v1alpha

import (
	"context"
	"regexp"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	koptanv1alpha "github.com/felukka/koptan/api/v1alpha"
)

// nolint:unused
// log is for logging in this package.
var dotnetAppLog = logf.Log.WithName("dotnetapp-resource")

// SetupDotnetAppWebhookWithManager registers the webhook for DotnetApp in the manager.
func SetupDotnetAppWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &koptanv1alpha.DotnetApp{}).
		WithValidator(&DotnetAppCustomValidator{}).
		WithDefaulter(&DotnetAppCustomDefaulter{}).
		Complete()
}

// DotnetAppCustomDefaulter struct is responsible for setting default values on the custom resource of the
// Kind DotnetApp when those are created or updated.
type DotnetAppCustomDefaulter struct{}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the Kind DotnetApp.
func (d *DotnetAppCustomDefaulter) Default(_ context.Context, obj *koptanv1alpha.DotnetApp) error {
	dotnetAppLog.Info("Defaulting for DotnetApp", "name", obj.GetName())

	// Default for SDKVersion
	if obj.Spec.SDKVersion == "" {
		obj.Spec.SDKVersion = "6.0"
	}

	// Default for Configuration
	if obj.Spec.Configuration == "" {
		obj.Spec.Configuration = "Release"
	}

	// Default for ExtraPackages
	if obj.Spec.ExtraPackages == nil {
		obj.Spec.ExtraPackages = []string{} // No default packages
	}

	// Default for Env
	if obj.Spec.Env == nil {
		obj.Spec.Env = map[string]string{} // Empty environment if not set
	}

	return nil
}

// DotnetAppCustomValidator struct is responsible for validating the DotnetApp resource
type DotnetAppCustomValidator struct{}

func (v *DotnetAppCustomValidator) ValidateCreate(_ context.Context, obj *koptanv1alpha.DotnetApp) (admission.Warnings, error) {
	dotnetAppLog.Info("Validation for DotnetApp upon creation", "name", obj.GetName())
	return nil, v.validateDotnetApp(obj)
}

func (v *DotnetAppCustomValidator) ValidateUpdate(_ context.Context, oldObj, newObj *koptanv1alpha.DotnetApp) (admission.Warnings, error) {
	dotnetAppLog.Info("Validation for DotnetApp upon update", "name", newObj.GetName())
	// Calling the same validation logic for updates
	return nil, v.validateDotnetApp(newObj)
}

func (v *DotnetAppCustomValidator) ValidateDelete(_ context.Context, obj *koptanv1alpha.DotnetApp) (admission.Warnings, error) {
	return nil, nil
}

func (v *DotnetAppCustomValidator) validateDotnetApp(obj *koptanv1alpha.DotnetApp) error {
	var allErrs field.ErrorList
	specPath := field.NewPath("spec")

	sourcePath := specPath.Child("source")
	if obj.Spec.Source.Repo == "" {
		allErrs = append(allErrs, field.Required(sourcePath.Child("url"), "source URL is required"))
	} else {
		if !strings.HasPrefix(obj.Spec.Source.Repo, "https://") {
			allErrs = append(allErrs, field.Invalid(sourcePath.Child("url"), obj.Spec.Source.Repo, "URL must use https:// protocol"))
		}
		if !strings.Contains(obj.Spec.Source.Repo, "@") {
			allErrs = append(allErrs, field.Invalid(sourcePath.Child("url"), obj.Spec.Source.Repo, "URL must contain an '@' symbol"))
		}
	}

	// Validate SDKVersion (if present)
	version := strings.TrimSpace(obj.Spec.SDKVersion)
	var versionRegex = regexp.MustCompile(`^\d+(\.\d+)*$`)

	// 2. Check against the regex (matches 1.0, 5.2.1, etc.)
	if !versionRegex.MatchString(version) {
		allErrs = append(allErrs, field.Invalid(
			specPath.Child("sdkVersion"),
			obj.Spec.SDKVersion,
			"Invalid SDK version format. Must be a numeric version (e.g., '1.2.3' or '5.0')",
		))
	}

	// Validate ProjectPath
	if obj.Spec.ProjectPath != "" {
		// Optionally validate the path format (e.g., it should be a relative path or within a directory)
		if !strings.HasPrefix(obj.Spec.ProjectPath, "/") {
			allErrs = append(allErrs, field.Invalid(specPath.Child("projectPath"), obj.Spec.ProjectPath, "ProjectPath must be an absolute path"))
		}
	}

	// Validate SelfContained (optional, if true, it must be set appropriately)
	if obj.Spec.SelfContained && obj.Spec.SDKVersion == "" {
		allErrs = append(allErrs, field.Required(specPath.Child("sdkVersion"), "sdkVersion is required when SelfContained is true"))
	}

	// Validate ExtraNugetSources (optional, valid URLs)
	if obj.Spec.ExtraNugetSources != nil {
		for _, source := range obj.Spec.ExtraNugetSources {
			if !strings.HasPrefix(source, "https://") {
				allErrs = append(allErrs, field.Invalid(specPath.Child("extraNugetSources"), source, "NuGet source URL must start with https://"))
			}
		}
	}

	// Validate Env (ensure no empty keys)
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
		schema.GroupKind{Group: "koptan.felukka.sh", Kind: "DotnetApp"},
		obj.Name,
		allErrs,
	)
}
