package languages

import (
	"github.com/felukka/koptan/internal/service/repofs"
)

// PHP serves the app with Apache and mod_php, installing Composer packages.
type PHP struct{}

const phpDefaultVersion = "8.3"

func (PHP) Name() string { return "php" }

func (PHP) Detect(fs *repofs.FS) (*Facts, bool) {
	f := newFacts("php", phpDefaultVersion)
	f.Extra["docroot"] = "/var/www/html"
	if fs.IsDir("public") {
		// Laravel, Symfony and Slim serve from public/.
		f.Extra["docroot"] = "/var/www/html/public"
	}
	f.Entrypoint = "index.php"
	switch {
	case fs.Exists("composer.json"):
		f.PackageManager = "composer"
		var composer struct {
			Require map[string]string `json:"require"`
		}
		if err := fs.ReadJSON("composer.json", &composer); err == nil {
			f.Version = majorMinor(composer.Require["php"], phpDefaultVersion)
		}
		f.InstallCmd = "composer install --no-dev --no-interaction --prefer-dist --optimize-autoloader"
		if fs.Exists("artisan") {
			f.Framework = "laravel"
		}
	case fs.Exists("index.php") || fs.Exists("public/index.php"):
	default:
		return f, false
	}
	return f, true
}
