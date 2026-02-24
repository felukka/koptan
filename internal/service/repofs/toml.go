package repofs

import (
	"bufio"
	"strings"
)

// TOMLValue returns the string value of key inside [section] of a TOML
// document; section "" means the top level. It understands only the flat
// `key = "value"` form, which is all detection needs from pyproject.toml,
// Cargo.toml and rust-toolchain.toml, and returns "" for anything else.
func TOMLValue(doc, section, key string) string {
	current := ""
	scanner := bufio.NewScanner(strings.NewReader(doc))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			current = strings.Trim(line, "[] ")
			continue
		}
		if current != section {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok || strings.TrimSpace(k) != key {
			continue
		}
		return unquote(strings.TrimSpace(v))
	}
	return ""
}

// TOMLHasSection reports whether doc has a [section] (or [[section]]) header.
func TOMLHasSection(doc, section string) bool {
	scanner := bufio.NewScanner(strings.NewReader(doc))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "[") && strings.Trim(line, "[] ") == section {
			return true
		}
	}
	return false
}

// unquote strips a basic or literal TOML string and a trailing comment.
func unquote(v string) string {
	if v == "" {
		return ""
	}
	q := v[0]
	if q != '"' && q != '\'' {
		if i := strings.Index(v, "#"); i >= 0 {
			v = v[:i]
		}
		return strings.TrimSpace(v)
	}
	if end := strings.IndexByte(v[1:], q); end >= 0 {
		return v[1 : end+1]
	}
	return ""
}
