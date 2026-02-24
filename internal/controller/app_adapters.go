package controller

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	koptan "github.com/felukka/koptan/api/v1alpha"
	"github.com/felukka/koptan/internal/appfactory"
)

type App interface {
	client.Object
	GetAppPhase() koptan.AppPhase
	SetAppPhase(koptan.AppPhase)
	GetObservedGeneration() int64
	SetObservedGeneration(int64)
	GetConfigMapName() string
	SetConfigMapName(string)
	GetError() string
	SetError(string)
	GetConditions() *[]metav1.Condition
	GetSourceRef() koptan.SourceRef
	RunDiscoveryAndGenerate(repoDir string) (string, error)
	RuntimeObject() client.Object
}

type goAppAdapter struct{ *koptan.GoApp }

func (a *goAppAdapter) GetAppPhase() koptan.AppPhase       { return a.Status.Phase }
func (a *goAppAdapter) SetAppPhase(p koptan.AppPhase)      { a.Status.Phase = p }
func (a *goAppAdapter) GetObservedGeneration() int64       { return a.Status.ObservedGeneration }
func (a *goAppAdapter) SetObservedGeneration(g int64)      { a.Status.ObservedGeneration = g }
func (a *goAppAdapter) GetConfigMapName() string           { return a.Status.ConfigMapName }
func (a *goAppAdapter) SetConfigMapName(n string)          { a.Status.ConfigMapName = n }
func (a *goAppAdapter) GetError() string                   { return a.Status.Error }
func (a *goAppAdapter) SetError(e string)                  { a.Status.Error = e }
func (a *goAppAdapter) GetConditions() *[]metav1.Condition { return &a.Status.Conditions }
func (a *goAppAdapter) GetSourceRef() koptan.SourceRef     { return a.Spec.Source }
func (a *goAppAdapter) RuntimeObject() client.Object       { return a.GoApp }

func (a *goAppAdapter) RunDiscoveryAndGenerate(repoDir string) (string, error) {
	info, err := appfactory.DiscoverGo(repoDir)
	if err != nil {
		return "", fmt.Errorf("go discovery failed: %w", err)
	}

	a.Status.DiscoveredGoVersion = info.GoVersion
	a.Status.DiscoveredEntrypoint = info.Entrypoint

	spec := a.Spec
	if spec.GoVersion == "" {
		spec.GoVersion = info.GoVersion
	}
	if spec.Entrypoint == "" {
		spec.Entrypoint = info.Entrypoint
	}

	return appfactory.GenerateGoApp(spec)
}

type dotnetAppAdapter struct{ *koptan.DotnetApp }

func (a *dotnetAppAdapter) GetAppPhase() koptan.AppPhase       { return a.Status.Phase }
func (a *dotnetAppAdapter) SetAppPhase(p koptan.AppPhase)      { a.Status.Phase = p }
func (a *dotnetAppAdapter) GetObservedGeneration() int64       { return a.Status.ObservedGeneration }
func (a *dotnetAppAdapter) SetObservedGeneration(g int64)      { a.Status.ObservedGeneration = g }
func (a *dotnetAppAdapter) GetConfigMapName() string           { return a.Status.ConfigMapName }
func (a *dotnetAppAdapter) SetConfigMapName(n string)          { a.Status.ConfigMapName = n }
func (a *dotnetAppAdapter) GetError() string                   { return a.Status.Error }
func (a *dotnetAppAdapter) SetError(e string)                  { a.Status.Error = e }
func (a *dotnetAppAdapter) GetConditions() *[]metav1.Condition { return &a.Status.Conditions }
func (a *dotnetAppAdapter) GetSourceRef() koptan.SourceRef     { return a.Spec.Source }
func (a *dotnetAppAdapter) RuntimeObject() client.Object       { return a.DotnetApp }

func (a *dotnetAppAdapter) RunDiscoveryAndGenerate(repoDir string) (string, error) {
	info, err := appfactory.DiscoverDotnet(repoDir)
	if err != nil {
		return "", fmt.Errorf("dotnet discovery failed: %w", err)
	}

	a.Status.DiscoveredSDKVersion = info.SDKVersion
	a.Status.DiscoveredProjectPath = info.ProjectPath

	spec := a.Spec
	if spec.SDKVersion == "" {
		spec.SDKVersion = info.SDKVersion
	}
	if spec.ProjectPath == "" {
		spec.ProjectPath = info.ProjectPath
	}

	return appfactory.GenerateDotnetApp(spec)
}

type javaAppAdapter struct{ *koptan.JavaApp }

func (a *javaAppAdapter) GetAppPhase() koptan.AppPhase       { return a.Status.Phase }
func (a *javaAppAdapter) SetAppPhase(p koptan.AppPhase)      { a.Status.Phase = p }
func (a *javaAppAdapter) GetObservedGeneration() int64       { return a.Status.ObservedGeneration }
func (a *javaAppAdapter) SetObservedGeneration(g int64)      { a.Status.ObservedGeneration = g }
func (a *javaAppAdapter) GetConfigMapName() string           { return a.Status.ConfigMapName }
func (a *javaAppAdapter) SetConfigMapName(n string)          { a.Status.ConfigMapName = n }
func (a *javaAppAdapter) GetError() string                   { return a.Status.Error }
func (a *javaAppAdapter) SetError(e string)                  { a.Status.Error = e }
func (a *javaAppAdapter) GetConditions() *[]metav1.Condition { return &a.Status.Conditions }
func (a *javaAppAdapter) GetSourceRef() koptan.SourceRef     { return a.Spec.Source }
func (a *javaAppAdapter) RuntimeObject() client.Object       { return a.JavaApp }

func (a *javaAppAdapter) RunDiscoveryAndGenerate(repoDir string) (string, error) {
	info, err := appfactory.DiscoverJava(repoDir)
	if err != nil {
		return "", fmt.Errorf("java discovery failed: %w", err)
	}

	a.Status.DiscoveredJavaVersion = info.JavaVersion
	a.Status.DiscoveredBuildTool = info.BuildTool
	a.Status.DiscoveredArtifactPath = info.ArtifactPath

	spec := a.Spec
	if spec.JavaVersion == "" {
		spec.JavaVersion = info.JavaVersion
	}
	if spec.BuildTool == "" {
		spec.BuildTool = info.BuildTool
	}
	if spec.ArtifactPath == "" {
		spec.ArtifactPath = info.ArtifactPath
	}

	return appfactory.GenerateJavaApp(spec)
}
