package appfactory

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	koptan "github.com/felukka/koptan/api/v1alpha"
)

type DotnetInfo struct {
	SDKVersion  string
	ProjectPath string
}

func DiscoverDotnet(repoDir string) (*DotnetInfo, error) {
	info := &DotnetInfo{}

	var csprojPath string
	_ = filepath.Walk(repoDir, func(path string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() {
			return nil
		}
		if strings.HasSuffix(fi.Name(), ".csproj") && csprojPath == "" {
			rel, _ := filepath.Rel(repoDir, path)
			csprojPath = rel
		}
		return nil
	})

	if csprojPath == "" {
		return nil, fmt.Errorf("no .csproj file found in %s", repoDir)
	}
	info.ProjectPath = csprojPath

	f, err := os.Open(filepath.Join(repoDir, csprojPath))
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = f.Close()
	}()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.Contains(line, "<TargetFramework>") {
			val := extractXMLValue(line, "TargetFramework")
			info.SDKVersion = frameworkToSDK(val)
			break
		}
	}

	if info.SDKVersion == "" {
		globalJson := filepath.Join(repoDir, "global.json")
		if data, err := os.ReadFile(globalJson); err == nil {
			for line := range strings.SplitSeq(string(data), "\n") {
				if strings.Contains(line, "version") {
					parts := strings.SplitN(line, ":", 2)
					if len(parts) == 2 {
						ver := strings.Trim(strings.TrimSpace(parts[1]), `",`)
						if ver != "" {
							info.SDKVersion = ver
						}
					}
				}
			}
		}
	}

	if info.SDKVersion == "" {
		info.SDKVersion = "8.0"
	}

	return info, nil
}

func extractXMLValue(line, tag string) string {
	start := "<" + tag + ">"
	end := "</" + tag + ">"
	i := strings.Index(line, start)
	if i < 0 {
		return ""
	}
	j := strings.Index(line, end)
	if j < 0 {
		return ""
	}
	return line[i+len(start) : j]
}

func frameworkToSDK(framework string) string {
	framework = strings.ToLower(framework)

	switch {
	case strings.Contains(framework, "9.0"):
		return "9.0"
	case strings.Contains(framework, "8.0"):
		return "8.0"
	case strings.Contains(framework, "7.0"):
		return "7.0"
	case strings.Contains(framework, "6.0"):
		return "6.0"
	}

	return ""
}

func GenerateDotnetApp(spec koptan.DotnetAppSpec) (string, error) {
	if spec.SDKVersion == "" {
		return "", fmt.Errorf("sdkVersion is required")
	}
	if spec.ProjectPath == "" {
		return "", fmt.Errorf("projectPath is required")
	}

	configuration := spec.Configuration
	if configuration == "" {
		configuration = "Release"
	}

	var runtimeImage string
	if spec.SelfContained {
		runtimeImage = fmt.Sprintf("mcr.microsoft.com/dotnet/runtime-deps:%s", spec.SDKVersion)
	} else {
		runtimeImage = fmt.Sprintf("mcr.microsoft.com/dotnet/aspnet:%s", spec.SDKVersion)
	}

	base := filepath.Base(spec.ProjectPath)
	assemblyName := strings.TrimSuffix(base, filepath.Ext(base))
	projectDir := filepath.Dir(spec.ProjectPath)

	var b strings.Builder

	fmt.Fprintf(&b, "FROM mcr.microsoft.com/dotnet/sdk:%s AS builder\n\n", spec.SDKVersion)

	if len(spec.ExtraPackages) > 0 {
		fmt.Fprintf(&b, "RUN apt-get update \\\n")
		fmt.Fprintf(
			&b,
			" && apt-get install -y --no-install-recommends %s \\\n",
			strings.Join(spec.ExtraPackages, " "),
		)
		fmt.Fprintf(&b, " && rm -rf /var/lib/apt/lists/*\n\n")
	}

	fmt.Fprintf(&b, "WORKDIR /src\n\n")

	for k, v := range spec.Env {
		fmt.Fprintf(&b, "ENV %s=%q\n", k, v)
	}
	if len(spec.Env) > 0 {
		b.WriteString("\n")
	}

	for _, src := range spec.ExtraNugetSources {
		fmt.Fprintf(&b, "RUN dotnet nuget add source %q\n", src)
	}
	if len(spec.ExtraNugetSources) > 0 {
		b.WriteString("\n")
	}

	fmt.Fprintf(&b, "COPY %s %s/\n", spec.ProjectPath, projectDir)
	fmt.Fprintf(&b, "COPY Directory.Build.props* ./\n")
	fmt.Fprintf(&b, "COPY Directory.Packages.props* ./\n")
	fmt.Fprintf(&b, "COPY nuget.config* ./\n\n")

	fmt.Fprintf(&b, "RUN dotnet restore %q\n\n", spec.ProjectPath)

	fmt.Fprintf(&b, "COPY . .\n\n")

	publishParts := []string{
		"dotnet publish",
		fmt.Sprintf("%q", spec.ProjectPath),
		"-c", configuration,
		"-o", "/out",
	}
	if spec.SelfContained {
		publishParts = append(publishParts, "--self-contained", "true", "-r", "linux-x64")
	} else {
		publishParts = append(publishParts, "--no-self-contained")
	}
	publishParts = append(publishParts, spec.BuildArgs...)
	fmt.Fprintf(&b, "RUN %s\n\n", strings.Join(publishParts, " "))

	fmt.Fprintf(&b, "FROM %s\n\n", runtimeImage)

	fmt.Fprintf(&b, "WORKDIR /app\n\n")

	fmt.Fprintf(&b, "COPY --from=builder /out .\n\n")

	fmt.Fprintf(&b, "ENTRYPOINT [\"dotnet\", \"%s.dll\"]\n", assemblyName)

	return b.String(), nil
}
