/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha

import (
	"context"

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
var voyagelog = logf.Log.WithName("voyage-resource")

// SetupVoyageWebhookWithManager registers the webhook for Voyage in the manager.
func SetupVoyageWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&koptanv1alpha.JavaApp{}).
		WithValidator(&JavaAppCustomValidator{}).
		WithDefaulter(&JavaAppCustomDefaulter{}).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// +kubebuilder:webhook:path=/mutate-koptan-felukka-sh-v1alpha-voyage,mutating=true,failurePolicy=fail,sideEffects=None,groups=koptan.felukka.sh,resources=voyages,verbs=create;update,versions=v1alpha,name=mvoyage-v1alpha.kb.io,admissionReviewVersions=v1

// VoyageCustomDefaulter struct is responsible for setting default values on the custom resource of the
// Kind Voyage when those are created or updated.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as it is used only for temporary operations and does not need to be deeply copied.
type VoyageCustomDefaulter struct {
	// TODO(user): Add more fields as needed for defaulting
}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the Kind Voyage.
func (d *VoyageCustomDefaulter) Default(ctx context.Context, obj runtime.Object) error {
	// Type assertion to ensure that obj is a *Voyage
	voyage, ok := obj.(*koptanv1alpha.Voyage)
	if !ok {
		return apierrors.NewBadRequest("expected Voyage object")
	}

	// Log the defaulting action
	voyagelog.Info("Defaulting for Voyage", "name", voyage.GetName())

	// Default resources if not set
	if voyage.Spec.Resources == nil {
		voyage.Spec.Resources = &koptanv1alpha.Resources{}
	}

	return nil
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// NOTE: If you want to customise the 'path', use the flags '--defaulting-path' or '--validation-path'.
// +kubebuilder:webhook:path=/validate-koptan-felukka-sh-v1alpha-voyage,mutating=false,failurePolicy=fail,sideEffects=None,groups=koptan.felukka.sh,resources=voyages,verbs=create;update,versions=v1alpha,name=vvoyage-v1alpha.kb.io,admissionReviewVersions=v1

// VoyageCustomValidator struct is responsible for validating the Voyage resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type VoyageCustomValidator struct {
	// TODO(user): Add more fields as needed for validation
}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type Voyage.
func (v *VoyageCustomValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	// Type assertion to ensure that obj is a *Voyage
	voyage, ok := obj.(*koptanv1alpha.Voyage)
	if !ok {
		return nil, apierrors.NewBadRequest("expected Voyage object")
	}

	// Log the validation action
	voyagelog.Info("Validation for Voyage upon creation", "name", voyage.GetName())

	// Call the validation logic for Voyage
	return nil, v.validateVoyage(voyage)
}

func (v *VoyageCustomValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	// Type assertion to ensure that newObj is a *Voyage
	newVoyage, ok := newObj.(*koptanv1alpha.Voyage)
	if !ok {
		return nil, apierrors.NewBadRequest("expected Voyage object")
	}

	// Log the validation action
	voyagelog.Info("Validation for Voyage upon update", "name", newVoyage.GetName())

	// Call the same validation logic for updates
	return nil, v.validateVoyage(newVoyage)
}

func (v *VoyageCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	// Type assertion to ensure that obj is a *Voyage
	voyage, ok := obj.(*koptanv1alpha.Voyage)
	if !ok {
		return nil, apierrors.NewBadRequest("expected Voyage object")
	}

	// Log the validation action
	voyagelog.Info("Validation for Voyage upon deletion", "name", voyage.GetName())

	// No specific validation required for delete
	return nil, nil
}

func (v *VoyageCustomValidator) validateVoyage(obj *koptanv1alpha.Voyage) error {
	var allErrs field.ErrorList
	specPath := field.NewPath("spec")

	if obj.Spec.SlipwayRef.Name == "" {
		allErrs = append(allErrs, field.Required(specPath.Child("slipwayRef").Child("name"), "slipwayRef.name is required"))
	}

	if obj.Spec.Port <= 0 || obj.Spec.Port > 65535 {
		allErrs = append(allErrs, field.Invalid(specPath.Child("port"), obj.Spec.Port, "Port must be between 1 and 65535"))
	}

	if obj.Spec.HealthCheck != nil && obj.Spec.HealthCheck.Path == "" {
		allErrs = append(allErrs, field.Invalid(specPath.Child("healthCheck").Child("path"), obj.Spec.HealthCheck.Path, "HealthCheck path cannot be empty"))
	}

	if obj.Spec.Resources != nil {
		if obj.Spec.Resources.CPURequest != nil && obj.Spec.Resources.CPURequest.Sign() <= 0 {
			allErrs = append(allErrs, field.Invalid(specPath.Child("resources").Child("cpuRequest"), obj.Spec.Resources.CPURequest, "CPURequest must be positive"))
		}

		if obj.Spec.Resources.MemoryRequest != nil && obj.Spec.Resources.MemoryRequest.Sign() <= 0 {
			allErrs = append(allErrs, field.Invalid(specPath.Child("resources").Child("memoryRequest"), obj.Spec.Resources.MemoryRequest, "MemoryRequest must be positive"))
		}
	}

	if obj.Spec.Env != nil {
		envPath := specPath.Child("env")
		for _, envVar := range obj.Spec.Env {
			if envVar.Name == "" {
				allErrs = append(allErrs, field.Invalid(envPath, envVar.Name, "Environment variable names cannot be empty"))
			}
		}
	}

	if len(allErrs) == 0 {
		return nil
	}

	return apierrors.NewInvalid(
		schema.GroupKind{Group: "koptan.felukka.sh", Kind: "Voyage"},
		obj.Name,
		allErrs,
	)
}
