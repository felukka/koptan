package languages

import (
	"github.com/felukka/koptan/internal/service/repofs"
)

// Static serves plain HTML with an unprivileged nginx.
type Static struct{}

func (Static) Name() string { return "static" }

func (Static) Detect(fs *repofs.FS) (*Facts, bool) {
	f := newFacts("static", "1.27")
	f.PackageManager = "none"
	f.Extra["root"] = "."
	for _, dir := range []string{".", "public", "site", "www", "html"} {
		if fs.Exists(dir + "/index.html") {
			f.Extra["root"] = dir
			f.Entrypoint = dir + "/index.html"
			return f, true
		}
	}
	f.Entrypoint = "index.html"
	return f, false
}
