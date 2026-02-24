#!/usr/bin/env bash
# Checks that <version> may be released for <component> at commit <sha> and
# prints, for $GITHUB_OUTPUT:
#   tag=<tag>            vX.Y.Z (operator), agent/vX.Y.Z or ui/vX.Y.Z
#   previous=<tag>       the component's latest other tag (release notes start)
#   existing=true|false  the tag already points at <sha> (pushed tag or re-run)
#
#   hack/release-version.sh <operator|agent|ui> <X.Y.Z> <sha>
#
# The version must be strictly higher than every other tag of the component.
# agent and ui also count the plain vX.Y.Z tags up to $shared_until, from
# before components were released separately (all three images were
# published under them), so they never reuse a published image tag.
# Needs the tags fetched (actions/checkout with fetch-depth: 0).
set -euo pipefail

component="${1:?usage: release-version.sh <operator|agent|ui> <X.Y.Z> <sha>}"
version="${2:?usage: release-version.sh <operator|agent|ui> <X.Y.Z> <sha>}"
sha="${3:?usage: release-version.sh <operator|agent|ui> <X.Y.Z> <sha>}"

case "$component" in
  operator) prefix="" ;;
  agent) prefix="agent/" ;;
  ui) prefix="ui/" ;;
  *)
    echo "unknown component \"$component\" (operator, agent or ui)" >&2
    exit 1
    ;;
esac
if [[ ! "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "version \"$version\" is not of the form X.Y.Z" >&2
  exit 1
fi
tag="${prefix}v${version}"
# The last release that tagged all three components with plain vX.Y.Z.
shared_until=0.0.4

existing=false
if at="$(git rev-parse -q --verify "refs/tags/$tag^{commit}")"; then
  if [ "$at" != "$(git rev-parse "$sha^{commit}")" ]; then
    echo "tag $tag already exists at $at, not at $sha" >&2
    exit 1
  fi
  existing=true
fi

# Every other X.Y.Z this component has used, as "X.Y.Z<TAB>tag".
others() {
  {
    git tag -l "${prefix}v*"
    if [ -n "$prefix" ]; then git tag -l 'v*'; fi
  } | while read -r t; do
    v="${t#"$prefix"}"
    v="${v#v}"
    if [[ ! "$v" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || [ "$t" = "$tag" ]; then
      continue
    fi
    # A plain tag after the split belongs to the operator only.
    if [ -n "$prefix" ] && [ "$t" = "v$v" ] &&
      [ "$(printf '%s\n%s\n' "$v" "$shared_until" | sort -V | tail -n1)" != "$shared_until" ]; then
      continue
    fi
    printf '%s\t%s\n' "$v" "$t"
  done | sort -V -k1,1
}
latest="$(others | tail -n1)"
if [ -n "$latest" ]; then
  latest_version="${latest%%$'\t'*}"
  if [ "$(printf '%s\n%s\n' "$latest_version" "$version" | sort -V | tail -n1)" != "$version" ] ||
    [ "$latest_version" = "$version" ]; then
    echo "$component $version is not higher than $latest_version (${latest#*$'\t'})" >&2
    exit 1
  fi
fi

# Release notes start at the component's own previous tag, or the last
# shared vX.Y.Z tag before its first separate release.
previous="$(others | awk -F'\t' -v p="$prefix" 'p == "" || index($2, p) == 1 { t = $2 } END { print t }')"
if [ -z "$previous" ]; then
  previous="${latest#*$'\t'}"
fi

echo "tag=$tag"
echo "previous=$previous"
echo "existing=$existing"
