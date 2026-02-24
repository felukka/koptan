package languages

import (
	"regexp"
	"strings"
)

var (
	majorMinorRe = regexp.MustCompile(`(\d+)\.(\d+)`)
	majorRe      = regexp.MustCompile(`\d+`)
)

// majorMinor extracts "X.Y" from a version constraint such as ">=3.11,<4"
// or "go 1.22.3"; it returns fallback when there is none.
func majorMinor(constraint, fallback string) string {
	if m := majorMinorRe.FindStringSubmatch(constraint); m != nil {
		return m[1] + "." + m[2]
	}
	return fallback
}

// major extracts the first whole number from a constraint such as
// "^20.11" or "v22"; it returns fallback when there is none.
func major(constraint, fallback string) string {
	if m := majorRe.FindString(constraint); m != "" {
		return m
	}
	return fallback
}

// firstLine returns the first non-empty, non-comment line of s.
func firstLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			return line
		}
	}
	return ""
}

// shellCmd runs command through sh so it can expand $PORT, exec'ing it so
// the application receives signals as PID 1.
func shellCmd(command string) []string {
	return []string{"sh", "-c", "exec " + command}
}
