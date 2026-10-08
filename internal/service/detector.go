package service

import (
	"os"
	"path/filepath"
	"strings"
)

// Detector identifies a language by inspecting the filesystem of a cloned repo.
type Detector interface {
	// Match returns true if the repo at rootDir contains evidence of this language.
	Match(rootDir string) bool
	// Language returns the human-readable language name.
	Language() string
}

// ---------------------------------------------------------------------------
// Language detectors
// ---------------------------------------------------------------------------

// GoDetector detects Go projects via go.mod presence.
type GoDetector struct{}

func (d *GoDetector) Match(rootDir string) bool {
	_, err := os.Stat(filepath.Join(rootDir, "go.mod"))
	return err == nil
}

func (d *GoDetector) Language() string { return "go" }

// JavaDetector detects Java/Maven projects via pom.xml or Gradle files.
type JavaDetector struct{}

func (d *JavaDetector) Match(rootDir string) bool {
	if _, err := os.Stat(filepath.Join(rootDir, "pom.xml")); err == nil {
		return true
	}
	// Check for Gradle.
	gradle := filepath.Join(rootDir, "build.gradle")
	gradleKts := filepath.Join(rootDir, "build.gradle.kts")
	if _, err := os.Stat(gradle); err == nil {
		return true
	}
	if _, err := os.Stat(gradleKts); err == nil {
		return true
	}
	return false
}

func (d *JavaDetector) Language() string { return "java" }

// DotnetDetector detects .NET projects via .csproj or .sln files.
type DotnetDetector struct{}

func (d *DotnetDetector) Match(rootDir string) bool {
	// Walk the root directory (one level) for solution or project files.
	entries, err := os.ReadDir(rootDir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		name := strings.ToLower(e.Name())
		if strings.HasSuffix(name, ".sln") || strings.HasSuffix(name, ".csproj") {
			return true
		}
	}
	return false
}

func (d *DotnetDetector) Language() string { return "dotnet" }

// NodeDetector detects Node.js projects via package.json presence.
type NodeDetector struct{}

func (d *NodeDetector) Match(rootDir string) bool {
	_, err := os.Stat(filepath.Join(rootDir, "package.json"))
	return err == nil
}

func (d *NodeDetector) Language() string { return "node" }

// PythonDetector detects Python projects via requirements.txt or pyproject.toml.
type PythonDetector struct{}

func (d *PythonDetector) Match(rootDir string) bool {
	_, err := os.Stat(filepath.Join(rootDir, "requirements.txt"))
	if err == nil {
		return true
	}
	_, err = os.Stat(filepath.Join(rootDir, "pyproject.toml"))
	return err == nil
}

func (d *PythonDetector) Language() string { return "python" }

// DockerfileOnlyDetector is a fallback: the repo has a Dockerfile but we
// cannot determine the source language from it. The reconciler still creates
// the CI CRD so the build can proceed with the user's own Dockerfile.
type DockerfileOnlyDetector struct{}

func (d *DockerfileOnlyDetector) Match(rootDir string) bool {
	_, err := os.Stat(filepath.Join(rootDir, "Dockerfile"))
	return err == nil
}

func (d *DockerfileOnlyDetector) Language() string { return "dockerfile" }
