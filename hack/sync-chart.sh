#!/usr/bin/env bash
# Syncs a Helm chart in felukka/charts with this repo and, when anything
# changed, sets appVersion and bumps the chart version:
#   charts/koptan     CRDs and the ClusterRole rules from config/rbac
#   charts/koptan-ui  the ClusterRole rules from config/rbac/backstage_role.yaml
#
#   hack/sync-chart.sh <chart-dir> <version>
#
# Needs yq v4. Run from the repository root; <chart-dir> must be in a git checkout.
set -euo pipefail

chart="${1:?usage: sync-chart.sh <chart-dir> <version>}"
version="${2:?usage: sync-chart.sh <chart-dir> <version>}"
version="${version#v}"

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

current="$(yq '.version' "$chart/Chart.yaml")"
app="$(yq '.appVersion' "$chart/Chart.yaml")"

if [ -z "$(git -C "$chart" status --porcelain -- .)" ] && [ "$app" = "$version" ]; then
  echo "$name chart is up to date with koptan $version"
  exit 0
fi

# Bump the chart patch version, or jump to the app version when it is higher.
IFS=. read -r major minor patch <<<"${current%%-*}"
next="$major.$minor.$((patch + 1))"
if [ "$(printf '%s\n%s\n' "$next" "$version" | sort -V | tail -n1)" = "$version" ]; then
  next="$version"
fi

VERSION="$version" NEXT="$next" yq -i '.appVersion = strenv(VERSION) | .version = strenv(NEXT)' \
  "$chart/Chart.yaml"
echo "$name chart $current -> $next (appVersion $version)"
