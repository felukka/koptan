// Package repofs is a read-only view of a cloned repository, rooted at the
// build context. Language detectors read the repository only through it, so
// a path in a repository (or a symlink) can never reach outside the clone.
package repofs

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// MaxFileSize bounds every read; manifests are small and anything larger is
// not worth parsing.
const MaxFileSize = 1 << 20

// ErrOutside is returned for a path that resolves outside the repository.
var ErrOutside = errors.New("path is outside the repository")

// FS is a read-only, sandboxed view of a directory.
type FS struct {
	root string
}

// New returns a view of dir. dir itself is resolved through symlinks once,
// so later containment checks compare real paths.
func New(dir string) (*FS, error) {
	real, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(real)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", dir)
	}
	return &FS{root: real}, nil
}

// Sub returns a view rooted at the relative directory rel.
func (f *FS) Sub(rel string) (*FS, error) {
	p, err := f.resolve(rel)
	if err != nil {
		return nil, err
	}
	return New(p)
}

// Root is the absolute path of the view.
func (f *FS) Root() string { return f.root }

// resolve maps a slash-separated relative path to a real path inside root.
func (f *FS) resolve(rel string) (string, error) {
	clean := path.Clean("/" + filepath.ToSlash(rel))
	full := filepath.Join(f.root, filepath.FromSlash(clean))
	real, err := filepath.EvalSymlinks(full)
	if err != nil {
		return "", err
	}
	if real != f.root && !strings.HasPrefix(real, f.root+string(filepath.Separator)) {
		return "", ErrOutside
	}
	return real, nil
}

// Exists reports whether rel exists inside the view.
func (f *FS) Exists(rel string) bool {
	_, err := f.resolve(rel)
	return err == nil
}

// IsDir reports whether rel is a directory inside the view.
func (f *FS) IsDir(rel string) bool {
	p, err := f.resolve(rel)
	if err != nil {
		return false
	}
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}

// Read returns the content of rel, at most MaxFileSize bytes.
func (f *FS) Read(rel string) ([]byte, error) {
	p, err := f.resolve(rel)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	data, err := io.ReadAll(io.LimitReader(file, MaxFileSize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > MaxFileSize {
		return nil, fmt.Errorf("%s is larger than %d bytes", rel, MaxFileSize)
	}
	return data, nil
}

// ReadString returns the content of rel, or "" when it cannot be read.
func (f *FS) ReadString(rel string) string {
	data, err := f.Read(rel)
	if err != nil {
		return ""
	}
	return string(data)
}

// ReadJSON decodes rel into v.
func (f *FS) ReadJSON(rel string, v any) error {
	data, err := f.Read(rel)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

// List returns the names of the entries of directory rel, sorted. dirs
// selects directories (true) or regular files (false).
func (f *FS) List(rel string, dirs bool) []string {
	p, err := f.resolve(rel)
	if err != nil {
		return nil
	}
	entries, err := os.ReadDir(p)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() == dirs {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out
}

// Glob returns the paths in directory rel whose base name matches pattern.
func (f *FS) Glob(rel, pattern string) []string {
	var out []string
	for _, name := range f.List(rel, false) {
		if ok, _ := path.Match(pattern, name); ok {
			out = append(out, path.Join(rel, name))
		}
	}
	return out
}

// FirstExisting returns the first of the candidates that exists, or "".
func (f *FS) FirstExisting(candidates ...string) string {
	for _, c := range candidates {
		if f.Exists(c) {
			return c
		}
	}
	return ""
}
