#!/usr/bin/env bash
# Installs isekai for the why-isekai GitHub Action.
set -euo pipefail
dir="${RUNNER_TEMP:-/tmp}/isekai-bin"
mkdir -p "$dir"
v="${ISEKAI_VERSION_INPUT:-latest}"
if [ "$v" = "source" ]; then
  command -v go >/dev/null || { echo "::error::version: source needs Go on the runner (actions/setup-go)"; exit 1; }
  exe=isekai; [ "${RUNNER_OS:-}" = "Windows" ] && exe=isekai.exe
  (cd "$ACTION_PATH" && CGO_ENABLED=0 go build -o "$dir/$exe" ./cmd/isekai)
else
  [ "$v" = "latest" ] && v=""
  ISEKAI_VERSION="$v" ISEKAI_INSTALL_DIR="$dir" sh "$ACTION_PATH/install.sh"
fi
echo "$dir" >> "${GITHUB_PATH:-/dev/null}"
"$dir/isekai" version
