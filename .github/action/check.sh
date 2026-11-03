#!/usr/bin/env bash
# Runs isekai check for the why-isekai GitHub Action.
set -uo pipefail
PATH="${RUNNER_TEMP:-/tmp}/isekai-bin:$PATH"
range="${INPUT_RANGE:-}"
zero="0000000000000000000000000000000000000000"
if [ -z "$range" ]; then
  if [ -n "${BASE_REF:-}" ]; then
    git fetch --no-tags --quiet origin "$BASE_REF" 2>/dev/null || true
    range="origin/$BASE_REF..HEAD"
  elif [ -n "${BEFORE:-}" ] && [ "$BEFORE" != "$zero" ] && git cat-file -e "$BEFORE^{commit}" 2>/dev/null; then
    range="$BEFORE..${AFTER:-HEAD}"
  else
    range="${AFTER:-HEAD}^!"
  fi
fi
if ! git rev-parse --verify -q "${range%%..*}^{commit}" >/dev/null 2>&1 && [[ "$range" == *..* ]]; then
  echo "::error::cannot resolve $range — use actions/checkout with fetch-depth: 0"
  exit 1
fi
args=("$range")
[ "${INPUT_STRICT:-false}" = "true" ] && args+=(--strict)
[ -n "${INPUT_MODE:-}" ] && export ISEKAI_MODE="$INPUT_MODE"
echo "checking $range"
isekai check "${args[@]}"
code=$?
if [ "${INPUT_SUMMARY:-true}" = "true" ] && [ -n "${GITHUB_STEP_SUMMARY:-}" ]; then
  isekai check "${args[@]}" --format md >> "$GITHUB_STEP_SUMMARY" || true
fi
failed=$(isekai check "${args[@]}" --format json 2>/dev/null | sed -n 's/^  "failed": \([0-9]*\),*/\1/p' | head -n 1)
echo "failed=${failed:-0}" >> "${GITHUB_OUTPUT:-/dev/null}"
exit $code
