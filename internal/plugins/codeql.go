package plugins

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	koptanv1 "github.com/felukka/koptan/api/v1"
	corev1 "k8s.io/api/core/v1"
)

const (
	codeQLImage     = "debian:bookworm-slim"
	codeQLBundleURL = "https://github.com/github/codeql-action/releases/latest/download/codeql-bundle-linux64.tar.gz"
)

// codeQLPacks maps CodeQL language names to their query pack.
var codeQLPacks = map[string]string{
	"javascript-typescript": "javascript", "javascript": "javascript", "typescript": "javascript",
	"python": "python", "java-kotlin": "java", "java": "java", "kotlin": "java",
	"csharp": "csharp", "ruby": "ruby", "go": "go", "c-cpp": "cpp", "cpp": "cpp", "swift": "swift",
}

// koptanToCodeQL maps a language Koptan detects to a CodeQL language.
var koptanToCodeQL = map[string]string{
	"go": "go", "node": "javascript-typescript", "python": "python",
	"java": "java-kotlin", "dotnet": "csharp", "ruby": "ruby",
}

var codeQLLangRe = regexp.MustCompile(`^[a-z+-]+$`)

// codeQLScript downloads the bundle, analyzes each language and fails when
// a result reaches $CODEQL_FAIL_ON. Inputs arrive only as environment
// variables; $CODEQL_LANGUAGES holds "language:pack" pairs.
const codeQLScript = `set -eu
if ! command -v curl >/dev/null 2>&1 || ! command -v jq >/dev/null 2>&1; then
  apt-get update -qq && apt-get install -y -qq --no-install-recommends curl jq ca-certificates >/dev/null
fi
if [ -z "$CODEQL_LANGUAGES" ]; then
  echo "koptan: no CodeQL language for $KOPTAN_LANGUAGE; skipping"
  exit 0
fi
echo "koptan: downloading the CodeQL bundle"
curl -fsSL "$CODEQL_BUNDLE_URL" | tar -xz -C /tmp
codeql=/tmp/codeql/codeql
mkdir -p "$KOPTAN_REPORTS_DIR"
found=0
for pair in $CODEQL_LANGUAGES; do
  lang="${pair%%:*}"; pack="${pair#*:}"
  case "$pack" in
    go|cpp|swift) mode=autobuild ;;
    *) mode=none ;;
  esac
  sarif="$KOPTAN_REPORTS_DIR/$KOPTAN_PLUGIN-$pack.sarif"
  "$codeql" database create "/tmp/db-$pack" --language="$lang" --source-root="$KOPTAN_SOURCE_DIR" \
    --build-mode="$mode" --overwrite
  "$codeql" database analyze "/tmp/db-$pack" \
    "codeql/$pack-queries:codeql-suites/$pack-$CODEQL_SUITE.qls" \
    --format=sarif-latest --output="$sarif"
  if [ -n "${GITHUB_TOKEN:-}" ]; then
    "$codeql" github upload-results --repository="$GITHUB_REPOSITORY" --ref="$GITHUB_REF" \
      --commit="$KOPTAN_REVISION" --sarif="$sarif"
  fi
  n=$(jq --arg min "$CODEQL_FAIL_ON" '
    {"error": 3, "warning": 2, "note": 1, "none": 0} as $rank
    | [.runs[].results[] | select($min != "none" and $rank[(.level // "warning")] >= $rank[$min])]
    | length' "$sarif")
  echo "koptan: $lang: $n result(s) at or above $CODEQL_FAIL_ON"
  found=$((found + n))
done
if [ "$found" -gt 0 ]; then
  echo "koptan: CodeQL found $found result(s) at or above $CODEQL_FAIL_ON"
  exit 1
fi
`

// CodeQL analyzes the source with the CodeQL CLI and fails on results at
// or above a severity. Languages without a build (or with build-mode none)
// work in the default image; go, cpp and swift need their toolchain.
type CodeQL struct{}

func (CodeQL) Validate(p *koptanv1.CIPlugin) error {
	cfg := p.Spec.CodeQL
	if cfg == nil {
		return errors.New("spec.codeql is required for type codeql")
	}
	for _, l := range cfg.Languages {
		if _, ok := codeQLPacks[l]; !ok || !codeQLLangRe.MatchString(l) {
			return fmt.Errorf("codeql language %q is not supported", l)
		}
	}
	if gh := cfg.GitHub; gh != nil && (gh.TokenSecretRef.Name == "" || gh.TokenSecretRef.Key == "") {
		return errors.New("spec.codeql.github.tokenSecretRef needs a name and a key")
	}
	return nil
}

func (CodeQL) Container(p *koptanv1.CIPlugin, env BuildEnv) (corev1.Container, error) {
	cfg := p.Spec.CodeQL
	image, bundle := cfg.Image, cfg.BundleURL
	if image == "" {
		image = codeQLImage
	}
	if bundle == "" {
		bundle = codeQLBundleURL
	}
	suite := cfg.QuerySuite
	if suite == "" {
		suite = "code-scanning"
	}
	failOn := string(cfg.FailOnSeverity)
	if failOn == "" {
		failOn = "error"
	}
	vars := []corev1.EnvVar{
		{Name: "CODEQL_LANGUAGES", Value: codeQLLanguages(cfg.Languages, env.Language)},
		{Name: "CODEQL_SUITE", Value: suite},
		{Name: "CODEQL_FAIL_ON", Value: failOn},
		{Name: "CODEQL_BUNDLE_URL", Value: bundle},
	}
	if gh := cfg.GitHub; gh != nil {
		vars = append(vars,
			corev1.EnvVar{Name: "GITHUB_REPOSITORY", Value: gh.Repository},
			corev1.EnvVar{Name: "GITHUB_REF", Value: gh.Ref},
			secretEnv("GITHUB_TOKEN", gh.TokenSecretRef),
		)
	}
	return corev1.Container{
		Image:   image,
		Command: []string{"sh", "-c", codeQLScript},
		Env:     vars,
	}, nil
}

func (CodeQL) Resources() corev1.ResourceRequirements {
	return requirements("500m", "2Gi", "4", "6Gi")
}

// codeQLLanguages renders "language:pack" pairs for the script: the
// configured languages, or the one matching the detected stack.
func codeQLLanguages(configured []string, detected string) string {
	langs := configured
	if len(langs) == 0 {
		if l, ok := koptanToCodeQL[detected]; ok {
			langs = []string{l}
		}
	}
	pairs := make([]string, 0, len(langs))
	for _, l := range langs {
		if pack, ok := codeQLPacks[l]; ok {
			pairs = append(pairs, l+":"+pack)
		}
	}
	return strings.Join(pairs, " ")
}
