#!/usr/bin/env bash
# Runs isekai check for the why-isekai GitHub Action.
set -uo pipefail
PATH="${RUNNER_TEMP:-/tmp}/isekai-bin:$PATH"
range="${INPUT_RANGE:-}"
# A shallow clone (the actions/checkout default) hides the commits being
# checked, and the check would pass without looking at them.
if [ "$(git rev-parse --is-shallow-repository 2>/dev/null)" = "true" ]; then
  echo "shallow clone: fetching full history (set fetch-depth: 0 on actions/checkout to skip this)"
  if ! git fetch --unshallow --no-tags --quiet origin 2>/dev/null; then
    echo "::error::the repository is a shallow clone and full history could not be fetched; use actions/checkout with fetch-depth: 0"
    exit 1
  fi
fi
zero="0000000000000000000000000000000000000000"
if [ -z "$range" ]; then
  if [ -n "${BASE_REF:-}" ]; then
    git fetch --no-tags --quiet origin "+refs/heads/$BASE_REF:refs/remotes/origin/$BASE_REF" 2>/dev/null || true
    range="origin/$BASE_REF..HEAD"
  elif [ -n "${BEFORE:-}" ] && [ "$BEFORE" != "$zero" ] && git cat-file -e "$BEFORE^{commit}" 2>/dev/null; then
    range="$BEFORE..${AFTER:-HEAD}"
  else
    range="${AFTER:-HEAD}^!"
    echo "::warning::no base to compare with (new branch or force push): checking only the last commit"
  fi
fi
if ! git rev-parse --verify -q "${range%%..*}^{commit}" >/dev/null 2>&1 && [[ "$range" == *..* ]]; then
  echo "::error::cannot resolve $range — use actions/checkout with fetch-depth: 0"
  exit 1
fi
commits=$(git rev-list --no-merges --count "$range" 2>/dev/null || echo 0)
if [ "$commits" = "0" ]; then
  if [ -n "${BASE_REF:-}" ]; then
    echo "::error::$range contains no commits to check; use actions/checkout with fetch-depth: 0"
    exit 1
  fi
  echo "::warning::$range contains no commits to check (merge commits are skipped)"
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
