#!/usr/bin/env bash
# Syncs a Helm chart in felukka/charts with this repo and, when anything
# changed, bumps the chart's patch version:
#   charts/koptan     CRDs and the ClusterRole rules from config/rbac
#   charts/koptan-ui  the ClusterRole rules from config/rbac/backstage_role.yaml
# --app-version sets appVersion (the operator version for koptan, the UI
# version for koptan-ui); --agent-version pins agent.image in koptan.
#
#   hack/sync-chart.sh <chart-dir> [--app-version X.Y.Z] [--agent-version X.Y.Z]
#
# Needs yq v4. Run from the repository root; <chart-dir> must be in a git checkout.
set -euo pipefail

usage="usage: sync-chart.sh <chart-dir> [--app-version X.Y.Z] [--agent-version X.Y.Z]"
chart="${1:?$usage}"
shift
app_version="" agent_version=""
while [ $# -gt 0 ]; do
  case "$1" in
    --app-version) app_version="${2:?$usage}"; shift 2 ;;
    --agent-version) agent_version="${2:?$usage}"; shift 2 ;;
    *) echo "$usage" >&2; exit 1 ;;
  esac
done
app_version="${app_version#v}"
agent_version="${agent_version#v}"

# replace_rules <template> <role.yaml> swaps the rules: block of a chart
# template (everything indented after "rules:") for the rules in role.yaml.
replace_rules() {
  local template="$1" role="$2" rules
  rules="$(yq '.rules' "$role" | sed 's/^/  /')"
  RULES="$rules" awk '
    skipping && /^[^ ]/ { skipping = 0 }
    skipping { next }
    { print }
    /^rules:$/ { print ENVIRON["RULES"]; skipping = 1 }
  ' "$template" >"$template.tmp"
  mv "$template.tmp" "$template"
}

name="$(yq '.name' "$chart/Chart.yaml")"
case "$name" in
  koptan)
    rm -f "$chart"/crds/*.yaml
    mkdir -p "$chart/crds"
    cp config/crd/bases/*.yaml "$chart/crds/"
    replace_rules "$chart/templates/clusterrole.yaml" config/rbac/role.yaml
    replace_rules "$chart/templates/backstage-clusterrole.yaml" config/rbac/backstage_role.yaml
    ;;
  koptan-ui)
    replace_rules "$chart/templates/clusterrole.yaml" config/rbac/backstage_role.yaml
    ;;
  *)
    echo "unknown chart $name" >&2
    exit 1
    ;;
esac

if [ -n "$app_version" ]; then
  VERSION="$app_version" yq -i '.appVersion = strenv(VERSION)' "$chart/Chart.yaml"
fi
if [ -n "$agent_version" ]; then
  if [ "$name" != koptan ]; then
    echo "--agent-version only applies to the koptan chart" >&2
    exit 1
  fi
  # sed, not yq: yq -i would reformat values.yaml and its comments.
  sed -i -E "s#^(  image: ghcr\.io/felukka/koptan-agent:).*#\1$agent_version#" "$chart/values.yaml"
  grep -qx "  image: ghcr.io/felukka/koptan-agent:$agent_version" "$chart/values.yaml" || {
    echo "agent.image not found in $chart/values.yaml" >&2
    exit 1
  }
fi

if [ -z "$(git -C "$chart" status --porcelain -- .)" ]; then
  echo "$name chart is up to date"
  exit 0
fi

current="$(yq '.version' "$chart/Chart.yaml")"
IFS=. read -r major minor patch <<<"${current%%-*}"
next="$major.$minor.$((patch + 1))"
# Jump to the app version when it is higher, as before.
if [ -n "$app_version" ] && [ "$(printf '%s\n%s\n' "$next" "$app_version" | sort -V | tail -n1)" = "$app_version" ]; then
  next="$app_version"
fi
NEXT="$next" yq -i '.version = strenv(NEXT)' "$chart/Chart.yaml"
echo "$name chart $current -> $next"
