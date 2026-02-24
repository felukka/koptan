package languages

import (
	"path"
	"regexp"
	"strings"

	"github.com/felukka/koptan/internal/service/repofs"
)

// Dotnet publishes a .NET project and runs it on the ASP.NET runtime image.
type Dotnet struct{}

const dotnetDefaultVersion = "8.0"

var (
	dotnetTFMRe      = regexp.MustCompile(`<TargetFrameworks?>\s*net(\d+\.\d+)`)
	dotnetAssemblyRe = regexp.MustCompile(`<AssemblyName>\s*([^<\s]+)\s*</AssemblyName>`)
	dotnetWebSdkRe   = regexp.MustCompile(`Sdk="Microsoft\.NET\.Sdk\.Web"`)
	dotnetExeRe      = regexp.MustCompile(`<OutputType>\s*Exe\s*</OutputType>`)
)

func (Dotnet) Name() string { return "dotnet" }

func (Dotnet) Detect(fs *repofs.FS) (*Facts, bool) {
	f := newFacts("dotnet", dotnetDefaultVersion)
	f.PackageManager = "nuget"
	project := dotnetProject(fs)
	if project == "" {
		f.Entrypoint = "app"
		f.Extra["project"] = "."
		f.StartCmd = []string{"dotnet", "app.dll"}
		f.Extra["user"] = "$APP_UID"
		return f, len(fs.Glob(".", "*.sln")) > 0
	}
	doc := fs.ReadString(project)
	if m := dotnetTFMRe.FindStringSubmatch(doc); m != nil {
		f.Version = m[1]
	}
	assembly := strings.TrimSuffix(path.Base(project), ".csproj")
	if m := dotnetAssemblyRe.FindStringSubmatch(doc); m != nil {
		assembly = m[1]
	}
	f.Entrypoint = assembly
	f.Extra["project"] = project
	f.StartCmd = []string{"dotnet", assembly + ".dll"}
	// .NET 8+ images ship a non-root "app" user ($APP_UID); older ones do not.
	f.Extra["user"] = "$APP_UID"
	if atoi(major(f.Version, "8")) < 8 {
		f.Extra["user"] = "1001"
	}
	return f, true
}

// dotnetProject picks the project to publish among *.csproj files at the
// root, one level down and under src/: a web project first, then an exe.
func dotnetProject(fs *repofs.FS) string {
	candidates := fs.Glob(".", "*.csproj")
	for _, base := range []string{".", "src"} {
		for _, dir := range fs.List(base, true) {
			candidates = append(candidates, fs.Glob(path.Join(base, dir), "*.csproj")...)
		}
	}
	if len(candidates) == 0 {
		return ""
	}
	for _, re := range []*regexp.Regexp{dotnetWebSdkRe, dotnetExeRe} {
		for _, c := range candidates {
			if re.MatchString(fs.ReadString(c)) {
				return c
			}
		}
	}
	return candidates[0]
}
