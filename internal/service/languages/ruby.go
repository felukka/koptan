package languages

import (
	"regexp"
	"strings"

	"github.com/felukka/koptan/internal/service/repofs"
)

// Ruby installs gems with Bundler and runs Rails, a Rack app or a script.
type Ruby struct{}

const rubyDefaultVersion = "3.3"

var rubyGemfileVersionRe = regexp.MustCompile(`(?m)^\s*ruby\s+["']([^"']+)["']`)

func (Ruby) Name() string { return "ruby" }

func (Ruby) Detect(fs *repofs.FS) (*Facts, bool) {
	f := newFacts("ruby", rubyDefaultVersion)
	f.PackageManager = "bundler"
	gemfile := fs.ReadString("Gemfile")
	if gemfile == "" {
		return f, false
	}
	if v := firstLine(fs.ReadString(".ruby-version")); v != "" {
		f.Version = majorMinor(strings.TrimPrefix(v, "ruby-"), rubyDefaultVersion)
	} else if m := rubyGemfileVersionRe.FindStringSubmatch(gemfile); m != nil {
		f.Version = majorMinor(m[1], rubyDefaultVersion)
	}
	f.InstallCmd = "bundle config set --local without 'development test' && bundle install --jobs 4"
	switch {
	case fs.Exists("config/application.rb") || strings.Contains(gemfile, `"rails"`) ||
		strings.Contains(gemfile, `'rails'`):
		f.Framework = "rails"
		f.Entrypoint = "config.ru"
		f.Extra["rails"] = "true"
		f.StartCmd = shellCmd("bundle exec rails server -b 0.0.0.0 -p ${PORT}")
	case fs.Exists("config.ru"):
		f.Framework = "rack"
		f.Entrypoint = "config.ru"
		server := "rackup -o 0.0.0.0 -p ${PORT}"
		if strings.Contains(gemfile, "puma") {
			server = "puma -b tcp://0.0.0.0:${PORT}"
		}
		f.StartCmd = shellCmd("bundle exec " + server)
	default:
		f.Entrypoint = fs.FirstExisting("app.rb", "main.rb", "server.rb")
		if f.Entrypoint == "" {
			f.Entrypoint = "app.rb"
		}
		f.StartCmd = []string{"bundle", "exec", "ruby", f.Entrypoint}
	}
	return f, true
}
