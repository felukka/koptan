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

// nolint:unused
// log is for logging in this package.
var dotnetAppLog = logf.Log.WithName("dotnetapp-resource")

// SetupDotnetAppWebhookWithManager registers the webhook for DotnetApp in the manager.
func SetupDotnetAppWebhookWithManager(mgr ctrl.Manager) error {
<<<<<<< HEAD
	return ctrl.NewWebhookManagedBy(mgr, &koptanv1alpha.DotnetApp{}).
=======
	return ctrl.NewWebhookManagedBy(mgr).
		For(&koptanv1alpha.DotnetApp{}).
>>>>>>> tmp-original-31-03-26-02-51
		WithValidator(&DotnetAppCustomValidator{}).
		WithDefaulter(&DotnetAppCustomDefaulter{}).
		Complete()
}

// DotnetAppCustomDefaulter struct is responsible for setting default values on the custom resource of the
// Kind DotnetApp when those are created or updated.
<<<<<<< HEAD
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as it is used only for temporary operations and does not need to be deeply copied.
type DotnetAppCustomDefaulter struct {
	// TODO(user): Add more fields as needed for defaulting
}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the Kind DotnetApp.
func (d *DotnetAppCustomDefaulter) Default(_ context.Context, obj *koptanv1alpha.DotnetApp) error {
	dotnetapplog.Info("Defaulting for DotnetApp", "name", obj.GetName())
=======
type DotnetAppCustomDefaulter struct{}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the Kind DotnetApp.
func (d *DotnetAppCustomDefaulter) Default(ctx context.Context, obj runtime.Object) error {
	dotnetApp, ok := obj.(*koptanv1alpha.DotnetApp)
	if !ok {
		return apierrors.NewBadRequest("expected DotnetApp object")
	}
>>>>>>> tmp-original-31-03-26-02-51

	dotnetAppLog.Info("Defaulting for DotnetApp", "name", dotnetApp.GetName())

	if dotnetApp.Spec.Configuration == "" {
		dotnetApp.Spec.Configuration = "Release"
	}

	if dotnetApp.Spec.ExtraPackages == nil {
		dotnetApp.Spec.ExtraPackages = []string{}
	}

	if dotnetApp.Spec.Env == nil {
		dotnetApp.Spec.Env = map[string]string{}
	}

	return nil
}

// DotnetAppCustomValidator struct is responsible for validating the DotnetApp resource
type DotnetAppCustomValidator struct{}

<<<<<<< HEAD
// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type DotnetApp.
func (v *DotnetAppCustomValidator) ValidateCreate(_ context.Context, obj *koptanv1alpha.DotnetApp) (admission.Warnings, error) {
	dotnetapplog.Info("Validation for DotnetApp upon creation", "name", obj.GetName())

	// TODO(user): fill in your validation logic upon object creation.

	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type DotnetApp.
func (v *DotnetAppCustomValidator) ValidateUpdate(_ context.Context, oldObj, newObj *koptanv1alpha.DotnetApp) (admission.Warnings, error) {
	dotnetapplog.Info("Validation for DotnetApp upon update", "name", newObj.GetName())

	// TODO(user): fill in your validation logic upon object update.

	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type DotnetApp.
func (v *DotnetAppCustomValidator) ValidateDelete(_ context.Context, obj *koptanv1alpha.DotnetApp) (admission.Warnings, error) {
	dotnetapplog.Info("Validation for DotnetApp upon deletion", "name", obj.GetName())

	// TODO(user): fill in your validation logic upon object deletion.

=======
func (v *DotnetAppCustomValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	dotnetApp, ok := obj.(*koptanv1alpha.DotnetApp)
	if !ok {
		return nil, apierrors.NewBadRequest("expected DotnetApp object")
	}
	dotnetAppLog.Info("Validation for DotnetApp upon creation", "name", dotnetApp.GetName())
	return nil, v.validateDotnetApp(dotnetApp)
}

func (v *DotnetAppCustomValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	newDotnetApp, ok := newObj.(*koptanv1alpha.DotnetApp)
	if !ok {
		return nil, apierrors.NewBadRequest("expected DotnetApp object")
	}
	dotnetAppLog.Info("Validation for DotnetApp upon update", "name", newDotnetApp.GetName())
	// Calling the same validation logic for updates
	return nil, v.validateDotnetApp(newDotnetApp)
}

func (v *DotnetAppCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	dotnetApp, ok := obj.(*koptanv1alpha.DotnetApp)
	if !ok {
		return nil, apierrors.NewBadRequest("expected DotnetApp object")
	}
	dotnetAppLog.Info("Validation for DotnetApp upon deletion", "name", dotnetApp.GetName())
>>>>>>> tmp-original-31-03-26-02-51
	return nil, nil
}

func (v *DotnetAppCustomValidator) validateDotnetApp(obj *koptanv1alpha.DotnetApp) error {
	var allErrs field.ErrorList
	specPath := field.NewPath("spec")

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
		// Validate that the path is repo-relative and does not escape the repo
		projectPath := strings.TrimSpace(obj.Spec.ProjectPath)
		if strings.HasPrefix(projectPath, "/") {
			allErrs = append(
				allErrs,
				field.Invalid(
					specPath.Child("projectPath"),
					obj.Spec.ProjectPath,
					"projectPath must be a repo-relative path and must not start with '/'",
				),
			)
		}
		for _, segment := range strings.Split(projectPath, "/") {
			if segment == ".." {
				allErrs = append(
					allErrs,
					field.Invalid(
						specPath.Child("projectPath"),
						obj.Spec.ProjectPath,
						"projectPath must not contain '..' path segments",
					),
				)
				break
			}
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
