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
	// codeQLImage has python3, which the Python extractor needs even with
	// build-mode none, and apt for curl and jq.
	codeQLImage = "docker.io/library/python:3.12-slim-bookworm"
	// The bundle is pinned and checked: it is 700 MB and "latest" would
	// make results change between builds of the same commit.
	codeQLBundleURL    = "https://github.com/github/codeql-action/releases/download/codeql-bundle-v2.27.2/codeql-bundle-linux64.tar.gz"
	codeQLBundleSHA256 = "f002864be6dd8d5d7bdb123aaf7291ec8e291012bd9f70b6d2362f730db52aeb"
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

// codeQLCountFilter counts the SARIF results at or above $min. CodeQL
// leaves result.level unset and puts the severity on the rule
// (defaultConfiguration.level), found by rule.index in the tool component
// that rule.toolComponent.index names (the driver when absent), or by id.
const codeQLCountFilter = `
{"error": 3, "warning": 2, "note": 1, "none": 0} as $rank
| [.runs[]
   | . as $run
   | ([$run.tool.driver.rules[]?, $run.tool.extensions[]?.rules[]?]) as $all
   | .results[]?
   | (.ruleId // .rule.id) as $id
   | (if .rule.toolComponent.index != null
      then ($run.tool.extensions[.rule.toolComponent.index].rules // [])
      else ($run.tool.driver.rules // []) end) as $rules
   | (.rule.index // .ruleIndex) as $i
   | ((if $i != null then $rules[$i] else null end)
      // ([$all[] | select(.id == $id)] | first)) as $rule
   | (.level // $rule.defaultConfiguration.level // "warning")
   | select($min != "none" and $rank[.] >= $rank[$min])]
| length`

// codeQLScript downloads the bundle, analyzes each language and fails when
// a result reaches $CODEQL_FAIL_ON. Inputs arrive only as environment
// variables; $CODEQL_LANGUAGES holds "language:pack" pairs. It fails, never
// skips, when there is no language to analyze: a gate that can be passed
// by not running is no gate. The verdict is counted before the optional
// GitHub upload, whose failure is only a warning.
const codeQLScript = `set -eu
if [ -z "$CODEQL_LANGUAGES" ]; then
  echo "koptan: no CodeQL language for stack \"$KOPTAN_LANGUAGE\"; set spec.codeql.languages or detach the plugin"
  exit 1
fi
if ! command -v curl >/dev/null 2>&1 || ! command -v jq >/dev/null 2>&1; then
  apt-get update -qq && apt-get install -y -qq --no-install-recommends curl jq ca-certificates >/dev/null
fi
work="${CODEQL_WORK_DIR:-/tmp}"
echo "koptan: downloading the CodeQL bundle"
curl -fsSL --retry 3 --retry-all-errors -o "$work/codeql-bundle.tar.gz" "$CODEQL_BUNDLE_URL"
if [ -n "${CODEQL_BUNDLE_SHA256:-}" ]; then
  echo "$CODEQL_BUNDLE_SHA256  $work/codeql-bundle.tar.gz" | sha256sum -c -
fi
tar -xzf "$work/codeql-bundle.tar.gz" -C "$work"
rm -f "$work/codeql-bundle.tar.gz"
codeql="$work/codeql/codeql"
mkdir -p "$KOPTAN_REPORTS_DIR"
found=0
for pair in $CODEQL_LANGUAGES; do
  lang="${pair%%:*}"; pack="${pair#*:}"
  case "$pack" in
    go|cpp|swift) mode=autobuild ;;
    *) mode=none ;;
  esac
  sarif="$KOPTAN_REPORTS_DIR/$KOPTAN_PLUGIN-$pack.sarif"
  "$codeql" database create "$work/db-$pack" --language="$lang" --source-root="$KOPTAN_SOURCE_DIR" \
    --build-mode="$mode" --overwrite
  "$codeql" database analyze "$work/db-$pack" \
    "codeql/$pack-queries:codeql-suites/$pack-$CODEQL_SUITE.qls" \
    --format=sarif-latest --output="$sarif"
  n=$(jq --arg min "$CODEQL_FAIL_ON" '` + codeQLCountFilter + `' "$sarif")
  echo "koptan: $lang: $n result(s) at or above $CODEQL_FAIL_ON"
  found=$((found + n))
  if [ -n "${GITHUB_TOKEN:-}" ]; then
    "$codeql" github upload-results --repository="$GITHUB_REPOSITORY" --ref="$GITHUB_REF" \
      --commit="$KOPTAN_REVISION" --sarif="$sarif" ||
      echo "koptan: warning: SARIF upload to $GITHUB_REPOSITORY failed; the verdict above stands"
  fi
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
	image, bundle, sum := cfg.Image, cfg.BundleURL, ""
	if image == "" {
		image = codeQLImage
	}
	if bundle == "" {
		// A bundleURL override is not checked: its digest is unknown.
		bundle, sum = codeQLBundleURL, codeQLBundleSHA256
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
		{Name: "CODEQL_BUNDLE_SHA256", Value: sum},
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
