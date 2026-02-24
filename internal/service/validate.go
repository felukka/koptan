package service

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"strings"
)

// MaxDockerfileSize keeps the Dockerfile well inside a ConfigMap.
const MaxDockerfileSize = 256 << 10

// ValidateDockerfile checks that data looks like a Dockerfile that can be
// stored and built: not empty, not too large, with a FROM instruction.
func ValidateDockerfile(data []byte) error {
	if len(bytes.TrimSpace(data)) == 0 {
		return errors.New("empty Dockerfile")
	}
	if len(data) > MaxDockerfileSize {
		return fmt.Errorf("the Dockerfile is larger than %d bytes", MaxDockerfileSize)
	}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if len(line) >= 5 && strings.EqualFold(line[:5], "FROM ") {
			return nil
		}
	}
	return errors.New("no FROM instruction in the Dockerfile")
}
