package languages

import (
	"regexp"

	"github.com/felukka/koptan/internal/service/repofs"
)

// Rust builds a release binary with cargo and runs it on Debian slim.
type Rust struct{}

const rustDefaultVersion = "1"

var rustBinNameRe = regexp.MustCompile(`(?s)\[\[bin\]\][^\[]*?name\s*=\s*"([^"]+)"`)

func (Rust) Name() string { return "rust" }

func (Rust) Detect(fs *repofs.FS) (*Facts, bool) {
	f := newFacts("rust", rustDefaultVersion)
	f.PackageManager = "cargo"
	f.Entrypoint = "app"
	cargo := fs.ReadString("Cargo.toml")
	if cargo == "" {
		return f, false
	}
	if name := repofs.TOMLValue(cargo, "package", "name"); name != "" {
		f.Entrypoint = name
	}
	if m := rustBinNameRe.FindStringSubmatch(cargo); m != nil {
		f.Entrypoint = m[1]
	}
	if channel := rustToolchain(fs); channel != "" {
		f.Version = majorMinor(channel, rustDefaultVersion)
	}
	f.BuildCmd = "cargo build --release"
	if fs.Exists("Cargo.lock") {
		f.BuildCmd += " --locked"
	}
	f.StartCmd = []string{"/app/" + f.Entrypoint}
	return f, true
}

// rustToolchain reads the pinned channel from rust-toolchain(.toml).
func rustToolchain(fs *repofs.FS) string {
	if doc := fs.ReadString("rust-toolchain.toml"); doc != "" {
		return repofs.TOMLValue(doc, "toolchain", "channel")
	}
	return firstLine(fs.ReadString("rust-toolchain"))
}
