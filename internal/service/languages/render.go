package languages

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"
)

//go:embed templates/*.Dockerfile.tmpl
var templateFS embed.FS

var templates = template.Must(template.New("").Funcs(template.FuncMap{
	// json renders a value as JSON, for exec-form CMD and ENTRYPOINT.
	"json": func(v any) (string, error) {
		b, err := json.Marshal(v)
		return string(b), err
	},
}).ParseFS(templateFS, "templates/*.Dockerfile.tmpl"))

// Render turns Facts into a Dockerfile using the language's template.
func Render(f *Facts) ([]byte, error) {
	name := f.Language + ".Dockerfile.tmpl"
	if templates.Lookup(name) == nil {
		return nil, fmt.Errorf("no Dockerfile template for language %q", f.Language)
	}
	if f.Port == 0 {
		return nil, fmt.Errorf("render %s: port is not set", f.Language)
	}
	if err := checkSingleLine(f); err != nil {
		return nil, fmt.Errorf("render %s: %w", f.Language, err)
	}
	var buf bytes.Buffer
	if err := templates.ExecuteTemplate(&buf, name, f); err != nil {
		return nil, fmt.Errorf("render %s: %w", f.Language, err)
	}
	return buf.Bytes(), nil
}

// checkSingleLine rejects facts read from the repository that would add
// lines to the Dockerfile, such as a crafted package name with a newline.
func checkSingleLine(f *Facts) error {
	values := []string{f.Version, f.PackageManager, f.Framework, f.Entrypoint, f.InstallCmd, f.BuildCmd}
	values = append(values, f.StartCmd...)
	for _, v := range f.Extra {
		values = append(values, v)
	}
	for _, v := range values {
		if strings.ContainsAny(v, "\r\n") {
			return fmt.Errorf("detected value %q spans several lines", v)
		}
	}
	return nil
}
