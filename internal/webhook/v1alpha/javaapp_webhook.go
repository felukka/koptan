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
var javaapplog = logf.Log.WithName("javaapp-resource")

// SetupJavaAppWebhookWithManager registers the webhook for JavaApp in the manager.
func SetupJavaAppWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &koptanv1alpha.JavaApp{}).
		WithValidator(&JavaAppCustomValidator{}).
		WithDefaulter(&JavaAppCustomDefaulter{}).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// +kubebuilder:webhook:path=/mutate-koptan-felukka-sh-v1alpha-javaapp,mutating=true,failurePolicy=fail,sideEffects=None,groups=koptan.felukka.sh,resources=javaapps,verbs=create;update,versions=v1alpha,name=mjavaapp-v1alpha.kb.io,admissionReviewVersions=v1

// JavaAppCustomDefaulter struct is responsible for setting default values on the custom resource of the
// Kind JavaApp when those are created or updated.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as it is used only for temporary operations and does not need to be deeply copied.
type JavaAppCustomDefaulter struct {
	// TODO(user): Add more fields as needed for defaulting
}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the Kind JavaApp.
func (d *JavaAppCustomDefaulter) Default(_ context.Context, obj *koptanv1alpha.JavaApp) error {
	javaapplog.Info("Defaulting for JavaApp", "name", obj.GetName())

	if obj.Spec.JavaVersion == "" {
		obj.Spec.JavaVersion = "17"
	}

	if obj.Spec.BuildTool == "maven" && obj.Spec.MavenGoal == "" {
		obj.Spec.MavenGoal = "package"
	}

	if obj.Spec.BuildArgs == nil {
		obj.Spec.BuildArgs = []string{}
	}

	if obj.Spec.ExtraPackages == nil {
		obj.Spec.ExtraPackages = []string{}
	}

	if obj.Spec.Env == nil {
		obj.Spec.Env = map[string]string{}
	}

	return nil
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// NOTE: If you want to customise the 'path', use the flags '--defaulting-path' or '--validation-path'.
// +kubebuilder:webhook:path=/validate-koptan-felukka-sh-v1alpha-javaapp,mutating=false,failurePolicy=fail,sideEffects=None,groups=koptan.felukka.sh,resources=javaapps,verbs=create;update,versions=v1alpha,name=vjavaapp-v1alpha.kb.io,admissionReviewVersions=v1

// JavaAppCustomValidator struct is responsible for validating the JavaApp resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type JavaAppCustomValidator struct {
	// TODO(user): Add more fields as needed for validation
}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type JavaApp.
func (v *JavaAppCustomValidator) ValidateCreate(_ context.Context, obj *koptanv1alpha.JavaApp) (admission.Warnings, error) {
	javaapplog.Info("Validation for JavaApp upon creation", "name", obj.GetName())

	// TODO(user): fill in your validation logic upon object creation.

	return nil, v.validateJavaApp(obj)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type JavaApp.
func (v *JavaAppCustomValidator) ValidateUpdate(_ context.Context, oldObj, newObj *koptanv1alpha.JavaApp) (admission.Warnings, error) {
	javaapplog.Info("Validation for JavaApp upon update", "name", newObj.GetName())

	// TODO(user): fill in your validation logic upon object update.

	return nil, v.validateJavaApp(newObj)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type JavaApp.
func (v *JavaAppCustomValidator) ValidateDelete(_ context.Context, obj *koptanv1alpha.JavaApp) (admission.Warnings, error) {
	javaapplog.Info("Validation for JavaApp upon deletion", "name", obj.GetName())

	// TODO(user): fill in your validation logic upon object deletion.

	return nil, nil
}

func (v *JavaAppCustomValidator) validateJavaApp(obj *koptanv1alpha.JavaApp) error {
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
		obj.Spec.BuildTool != "maven" &&
		obj.Spec.BuildTool != "gradle" {

		allErrs = append(allErrs, field.Invalid(
			specPath.Child("buildTool"),
			obj.Spec.BuildTool,
			"must be either 'maven' or 'gradle'",
		))
	}

	if obj.Spec.ArtifactPath != "" {
		if !strings.HasPrefix(obj.Spec.ArtifactPath, "/") {
			allErrs = append(allErrs, field.Invalid(
				specPath.Child("artifactPath"),
				obj.Spec.ArtifactPath,
				"artifactPath must be an absolute path",
			))
		}
	}

	if obj.Spec.BuildTool == "maven" {
		if obj.Spec.GradleTask != "" && obj.Spec.GradleTask != "build" {
			allErrs = append(allErrs, field.Invalid(
				specPath.Child("gradleTask"),
				obj.Spec.GradleTask,
				"gradleTask should not be set when using maven",
			))
		}
	}

	if obj.Spec.BuildTool == "gradle" {
		if obj.Spec.MavenGoal != "" && obj.Spec.MavenGoal != "package" {
			allErrs = append(allErrs, field.Invalid(
				specPath.Child("mavenGoal"),
				obj.Spec.MavenGoal,
				"mavenGoal should not be set when using gradle",
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
