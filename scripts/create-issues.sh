#!/bin/sh
# Create GitHub issues from .github/issues/*.md (front matter: title, labels).
# Safe to re-run: an issue whose title already exists is skipped.
#   gh auth login && sh scripts/create-issues.sh [owner/repo]
set -eu
repo="${1:-bauyrzhanospan/why-isekai}"
dir="$(dirname "$0")/../.github/issues"
command -v gh >/dev/null 2>&1 || { echo "needs the GitHub CLI: https://cli.github.com" >&2; exit 1; }

existing=$(gh issue list -R "$repo" --state all --limit 500 --json title --jq '.[].title')
for f in "$dir"/*.md; do
  title=$(sed -n 's/^title: *"\(.*\)"$/\1/p' "$f" | head -n 1)
  labels=$(sed -n 's/^labels: *\[\(.*\)\]$/\1/p' "$f" | tr -d ' ')
  [ -n "$title" ] || { echo "skip $f: no title" >&2; continue; }
  if printf '%s\n' "$existing" | grep -Fqx "$title"; then echo "exists: $title"; continue; fi
  for l in $(printf '%s' "$labels" | tr ',' ' '); do
    gh label create "$l" -R "$repo" >/dev/null 2>&1 || true
  done
  status=$(sed -n 's/^status: *\([a-z]*\)$/\1/p' "$f" | head -n 1)
  body=$(awk 'BEGIN{n=0} /^---$/{n++; next} n>=2' "$f")
  url=$(gh issue create -R "$repo" --title "$title" --label "$labels" --body "$body")
  if [ "$status" = "done" ]; then
    gh issue close -R "$repo" "$url" --comment "Done before the repository went public; see the Status line." >/dev/null
    echo "created (closed, done): $title"
  else
    echo "created: $title"
  fi
done
