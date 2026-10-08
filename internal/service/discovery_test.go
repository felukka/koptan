package service

import (
	"strings"
	"testing"
)

func TestTemplatesListenOnPortAndCoverDetectors(t *testing.T) {
	for _, d := range NewEngine().Detectors {
		lang := d.Language()
		if lang == "dockerfile" {
			continue
		}
		tmpl, ok := languageDockerfiles[lang]
		if !ok {
			t.Errorf("no Dockerfile template for detected language %q", lang)
			continue
		}
		if !strings.Contains(tmpl, "PORT=8080") {
			t.Errorf("%s template does not set PORT", lang)
		}
	}
	if strings.Contains(languageDockerfiles["go"], "COPY go.mod go.sum") {
		t.Error("go template must not require go.sum")
	}
	if !strings.Contains(languageDockerfiles["java"], "FROM maven:") {
		t.Error("java template needs a Maven builder image")
	}
	if !strings.Contains(languageDockerfiles["dotnet"], "AssemblyName=app") ||
		strings.Contains(languageDockerfiles["dotnet"], "USER appuser") {
		t.Error("dotnet template must publish app.dll and run as the image's app user")
	}
}
