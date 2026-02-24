#!/usr/bin/env bash
# Fails when a tracked source file is longer than MAX_LINES (default 500).
# Generated files, lockfiles and vendored assets are skipped.
set -euo pipefail

max="${MAX_LINES:-500}"
status=0

while IFS= read -r file; do
  [ -f "$file" ] || continue
  lines=$(wc -l <"$file")
  if [ "$lines" -gt "$max" ]; then
    echo "$file: $lines lines (max $max)"
    status=1
  fi
done < <(git ls-files --cached --others --exclude-standard \
  '*.go' '*.ts' '*.tsx' '*.js' '*.mjs' '*.css' '*.sh' '*.tmpl' \
  ':!:**/zz_generated*' ':!:web/.yarn/**' ':!:**/dist/**' ':!:**/node_modules/**')

exit "$status"
