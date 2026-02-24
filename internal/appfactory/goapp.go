package appfactory

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	koptan "github.com/felukka/koptan/api/v1alpha"
)

type GoInfo struct {
	GoVersion  string
	Entrypoint string
}

func DiscoverGo(repoDir string) (*GoInfo, error) {
	info := &GoInfo{Entrypoint: "."}

	gomod := filepath.Join(repoDir, "go.mod")
	f, err := os.Open(gomod)
	if err != nil {
		return nil, fmt.Errorf("go.mod not found in %s", repoDir)
	}
	defer func() {
		_ = f.Close()
	}()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if after, found := strings.CutPrefix(line, "go "); found {
			info.GoVersion = after
			break
		}
	}

	if info.GoVersion == "" {
		return nil, fmt.Errorf("could not find go version in go.mod")
	}

	candidates := []string{"./cmd/main.go", "./main.go"}
	for _, c := range candidates {
		if _, err := os.Stat(filepath.Join(repoDir, c)); err == nil {
			info.Entrypoint = "./" + filepath.Dir(c)
			break
		}
	}

	cmdDir := filepath.Join(repoDir, "cmd")
	entries, err := os.ReadDir(cmdDir)
	if err == nil {
		for _, e := range entries {
			if e.IsDir() {
				mainGo := filepath.Join(cmdDir, e.Name(), "main.go")
				if _, err := os.Stat(mainGo); err == nil {
					info.Entrypoint = "./cmd/" + e.Name()
					break
				}
			}
		}
	}

	return info, nil
}

func GenerateGoApp(spec koptan.GoAppSpec) (string, error) {
	if spec.GoVersion == "" {
		return "", fmt.Errorf("goVersion is required")
	}

	entrypoint := spec.Entrypoint
	if entrypoint == "" {
		entrypoint = "."
	}

	const binaryName = "app"

	runtimeImage := "gcr.io/distroless/static-debian12:nonroot"
	if spec.CGOEnabled {
		runtimeImage = "gcr.io/distroless/base-debian12:nonroot"
	}

	cgoEnabled := "0"
	if spec.CGOEnabled {
		cgoEnabled = "1"
	}

	ldflags := spec.LDFlags
	if ldflags == "" {
		ldflags = "-s -w"
	}

	var b strings.Builder

	fmt.Fprintf(&b, "FROM golang:%s-alpine AS builder\n\n", spec.GoVersion)

	pkgs := append([]string{}, spec.ExtraPackages...)
	if spec.CGOEnabled {
		pkgs = appendUnique(pkgs, "gcc", "musl-dev")
	}
	if len(pkgs) > 0 {
		fmt.Fprintf(&b, "RUN apk add --no-cache %s\n\n", strings.Join(pkgs, " "))
	}

	fmt.Fprintf(&b, "WORKDIR /src\n\n")

	for k, v := range spec.Env {
		fmt.Fprintf(&b, "ENV %s=%q\n", k, v)
	}
	if len(spec.Env) > 0 {
		b.WriteString("\n")
	}

	fmt.Fprintf(&b, "COPY go.mod go.sum ./\n")
	fmt.Fprintf(&b, "RUN go mod download\n\n")

	fmt.Fprintf(&b, "COPY . .\n\n")

	fmt.Fprintf(&b, "ENV CGO_ENABLED=%s\n", cgoEnabled)

	buildParts := []string{
		"go build",
		fmt.Sprintf("-ldflags %q", ldflags),
	}
	buildParts = append(buildParts, spec.BuildArgs...)
	buildParts = append(buildParts, fmt.Sprintf("-o /out/%s", binaryName))
	buildParts = append(buildParts, entrypoint)
	fmt.Fprintf(&b, "RUN %s\n\n", strings.Join(buildParts, " "))

	fmt.Fprintf(&b, "FROM %s\n\n", runtimeImage)

	fmt.Fprintf(&b, "COPY --from=builder /out/%s /%s\n\n", binaryName, binaryName)

	fmt.Fprintf(&b, "ENTRYPOINT [\"/%s\"]\n", binaryName)

	return b.String(), nil
}

func appendUnique(slice []string, vals ...string) []string {
	set := make(map[string]struct{}, len(slice))
	for _, s := range slice {
		set[s] = struct{}{}
	}
	for _, v := range vals {
		if _, ok := set[v]; !ok {
			slice = append(slice, v)
			set[v] = struct{}{}
		}
	}
	return slice
}
