package v1alpha

import (
	"context"

<<<<<<< HEAD
=======
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
var slipwaylog = logf.Log.WithName("slipway-resource")

// SetupSlipwayWebhookWithManager registers the webhook for Slipway in the manager.
func SetupSlipwayWebhookWithManager(mgr ctrl.Manager) error {
<<<<<<< HEAD
	return ctrl.NewWebhookManagedBy(mgr, &koptanv1alpha.Slipway{}).
=======
	return ctrl.NewWebhookManagedBy(mgr).
		For(&koptanv1alpha.Slipway{}).
>>>>>>> tmp-original-31-03-26-02-51
		WithValidator(&SlipwayCustomValidator{}).
		WithDefaulter(&SlipwayCustomDefaulter{}).
		Complete()
}

type SlipwayCustomDefaulter struct {
	// TODO(user): Add more fields as needed for defaulting
}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the Kind Slipway.
<<<<<<< HEAD
func (d *SlipwayCustomDefaulter) Default(_ context.Context, obj *koptanv1alpha.Slipway) error {
	slipwaylog.Info("Defaulting for Slipway", "name", obj.GetName())
=======
func (d *SlipwayCustomDefaulter) Default(ctx context.Context, obj runtime.Object) error {
	// Type assertion to ensure that obj is a *Slipway
	slipway, ok := obj.(*koptanv1alpha.Slipway)
	if !ok {
		return apierrors.NewBadRequest("expected Slipway object")
	}

	// Log the defaulting action
	slipwaylog.Info("Defaulting for Slipway", "name", slipway.GetName())
>>>>>>> tmp-original-31-03-26-02-51

	// Default AppRef.Kind to "GoApp" if not set
	if slipway.Spec.AppRef.Kind == "" {
		slipway.Spec.AppRef.Kind = "GoApp"
	}

	// Default Image.Registry to "docker.io" if not set
	if slipway.Spec.Image.Registry == "" {
		slipway.Spec.Image.Registry = "docker.io"
	}

	return nil
}

type SlipwayCustomValidator struct {
	// TODO(user): Add more fields as needed for validation
}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type Slipway.
<<<<<<< HEAD
func (v *SlipwayCustomValidator) ValidateCreate(_ context.Context, obj *koptanv1alpha.Slipway) (admission.Warnings, error) {
	slipwaylog.Info("Validation for Slipway upon creation", "name", obj.GetName())
=======
func (v *SlipwayCustomValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	// Type assertion to ensure that obj is a *Slipway
	slipway, ok := obj.(*koptanv1alpha.Slipway)
	if !ok {
		return nil, apierrors.NewBadRequest("expected Slipway object")
	}

	// Log the validation action
	slipwaylog.Info("Validation for Slipway upon creation", "name", slipway.GetName())
>>>>>>> tmp-original-31-03-26-02-51

	// Call the validation logic for Slipway
	return nil, v.validateSlipway(slipway)
}

<<<<<<< HEAD
// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type Slipway.
func (v *SlipwayCustomValidator) ValidateUpdate(_ context.Context, oldObj, newObj *koptanv1alpha.Slipway) (admission.Warnings, error) {
	slipwaylog.Info("Validation for Slipway upon update", "name", newObj.GetName())
=======
func (v *SlipwayCustomValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	// Type assertion to ensure that newObj is a *Slipway
	newSlipway, ok := newObj.(*koptanv1alpha.Slipway)
	if !ok {
		return nil, apierrors.NewBadRequest("expected Slipway object")
	}
>>>>>>> tmp-original-31-03-26-02-51

	// Log the validation action
	slipwaylog.Info("Validation for Slipway upon update", "name", newSlipway.GetName())

	// Call the same validation logic for updates
	return nil, v.validateSlipway(newSlipway)
}

<<<<<<< HEAD
// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type Slipway.
func (v *SlipwayCustomValidator) ValidateDelete(_ context.Context, obj *koptanv1alpha.Slipway) (admission.Warnings, error) {
	slipwaylog.Info("Validation for Slipway upon deletion", "name", obj.GetName())
=======
func (v *SlipwayCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	// Type assertion to ensure that obj is a *Slipway
	slipway, ok := obj.(*koptanv1alpha.Slipway)
	if !ok {
		return nil, apierrors.NewBadRequest("expected Slipway object")
	}

	// Log the validation action
	slipwaylog.Info("Validation for Slipway upon deletion", "name", slipway.GetName())
>>>>>>> tmp-original-31-03-26-02-51

	// No specific validation required for delete
	return nil, nil
}

func (v *SlipwayCustomValidator) validateSlipway(obj *koptanv1alpha.Slipway) error {
	var allErrs field.ErrorList
	specPath := field.NewPath("spec")

	// 1. Validate AppRef.Name is not empty
	if obj.Spec.AppRef.Name == "" {
		allErrs = append(allErrs, field.Required(specPath.Child("appRef").Child("name"), "appRef.name is required"))
	}

	// 2. Validate AppRef.Kind is a valid value
	validKinds := []string{"GoApp", "DotnetApp", "JavaApp"}
	valid := false
	for _, kind := range validKinds {
		if obj.Spec.AppRef.Kind == kind {
			valid = true
			break
		}
	}
	if !valid {
		allErrs = append(allErrs, field.Invalid(specPath.Child("appRef").Child("kind"), obj.Spec.AppRef.Kind, fmt.Sprintf("appRef.kind must be one of: %v", validKinds)))
	}

	if obj.Spec.Image.Registry == "" {
		allErrs = append(allErrs, field.Required(specPath.Child("image").Child("registry"), "image.registry is required"))
	}

	if obj.Spec.Image.Name == "" {
		allErrs = append(allErrs, field.Required(specPath.Child("image").Child("name"), "image.name is required"))
	}

	if obj.Spec.ExtraSteps != nil {
		for i, step := range obj.Spec.ExtraSteps {
			if step.Name == "" {
				allErrs = append(allErrs, field.Invalid(specPath.Child("extraSteps").Index(i).Child("name"), step.Name, "container name must not be empty"))
			}
		}
	}

	if len(allErrs) == 0 {
		return nil
	}

	return apierrors.NewInvalid(
		schema.GroupKind{Group: "koptan.felukka.sh", Kind: "Slipway"},
		obj.Name,
		allErrs,
	)
}
