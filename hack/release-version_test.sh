#!/usr/bin/env bash
# Tests hack/release-version.sh against a throwaway repository with tags.
#   bash hack/release-version_test.sh
set -euo pipefail

script="$(cd "$(dirname "$0")" && pwd)/release-version.sh"
repo="$(mktemp -d)"
trap 'rm -rf "$repo"' EXIT
cd "$repo"
git init -q
git -c user.name=t -c user.email=t@t commit -q --allow-empty -m one
one="$(git rev-parse HEAD)"
git -c user.name=t -c user.email=t@t commit -q --allow-empty -m two
two="$(git rev-parse HEAD)"
git tag v0.0.3 "$one"
git tag v0.0.4 "$one"
git tag agent/v0.0.6 "$one"

fails=0
ok() { # ok <expected output line> <args...>
  local want="$1" out
  shift
  if ! out="$("$script" "$@" 2>&1)" || ! grep -qxF "$want" <<<"$out"; then
    echo "FAIL: $* -> want \"$want\", got: $out"
    fails=$((fails + 1))
  fi
}
refused() { # refused <args...>
  local out
  if out="$("$script" "$@" 2>&1)"; then
    echo "FAIL: $* should be refused, got: $out"
    fails=$((fails + 1))
  fi
}

ok tag=v0.0.5 operator 0.0.5 "$two"
ok previous=v0.0.4 operator 0.0.5 "$two"
ok existing=false operator 0.0.5 "$two"
refused operator 0.0.4 "$two"    # equal: re-released on another commit
refused operator 0.0.2 "$two"    # lower
refused operator 0.1 "$two"      # not X.Y.Z
refused nope 0.0.9 "$two"        # unknown component

# A tag already at this commit is a pushed tag or a re-run: allowed, and
# compared only with the other tags.
ok existing=true operator 0.0.4 "$one"
ok previous=v0.0.3 operator 0.0.4 "$one"

# agent and ui count the old shared tags and their own.
refused ui 0.0.4 "$two"
ok tag=ui/v0.0.5 ui 0.0.5 "$two"
ok previous=v0.0.4 ui 0.0.5 "$two"
refused agent 0.0.5 "$two"       # agent/v0.0.6 exists
ok tag=agent/v0.0.7 agent 0.0.7 "$two"
ok previous=agent/v0.0.6 agent 0.0.7 "$two"

# "all 0.0.5" checks each component: the agent refuses it.
ok tag=v0.0.5 operator 0.0.5 "$two"
refused agent 0.0.5 "$two"

# Plain tags after the last shared release (0.0.4) are the operator's only:
# they don't block or start agent/ui versions.
git tag v0.2.0 "$two"
ok tag=ui/v0.1.0 ui 0.1.0 "$two"
ok previous=v0.0.4 ui 0.1.0 "$two"
refused operator 0.1.0 "$two"

# 0.2.10 sorts above 0.2.9.
git tag ui/v0.2.9 "$one"
refused ui 0.2.8 "$two"
ok tag=ui/v0.2.10 ui 0.2.10 "$two"
ok previous=ui/v0.2.9 ui 0.2.10 "$two"

if [ "$fails" -gt 0 ]; then
  echo "$fails failure(s)"
  exit 1
fi
echo "release-version: all cases pass"
