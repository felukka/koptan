package service

import (
	"fmt"

	"github.com/felukka/koptan/internal/service/repofs"
)

// dockerfileNames are looked up in the build context, in this order.
var dockerfileNames = []string{
	"Dockerfile", "Containerfile", "dockerfile",
	"docker/Dockerfile", "build/Dockerfile", "deploy/Dockerfile",
}

// locateDockerfile returns the repository's own Dockerfile. An explicit
// path (relative to the repository root) must exist; otherwise the usual
// names are tried in the build context. found is false when there is none.
func locateDockerfile(root, context *repofs.FS, explicit string) (data []byte, found bool, err error) {
	if explicit != "" {
		data, err := root.Read(explicit)
		if err != nil {
			return nil, false, fmt.Errorf("spec.build.dockerfilePath %q: %w", explicit, err)
		}
		return data, true, nil
	}
	for _, name := range dockerfileNames {
		if context.IsDir(name) || !context.Exists(name) {
			continue
		}
		data, err := context.Read(name)
		if err != nil {
			return nil, false, fmt.Errorf("read %s: %w", name, err)
		}
		return data, true, nil
	}
	return nil, false, nil
}
