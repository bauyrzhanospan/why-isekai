#!/bin/sh
# Install isekai from GitHub Releases.
#   curl -fsSL https://raw.githubusercontent.com/bauyrzhanospan/why-isekai/main/install.sh | sh
# Options (env): ISEKAI_VERSION=0.1.0 (default: latest), ISEKAI_INSTALL_DIR=~/.local/bin,
#   ISEKAI_DOWNLOAD_BASE=<mirror URL holding the release assets>
set -eu

repo="bauyrzhanospan/why-isekai"
version="${ISEKAI_VERSION:-}"
dir="${ISEKAI_INSTALL_DIR:-$HOME/.local/bin}"

fail() { echo "isekai install: $*" >&2; exit 1; }

case "$(uname -s)" in
  Linux) os=linux ;;
  Darwin) os=darwin ;;
  MINGW*|MSYS*|CYGWIN*) os=windows ;;
  *) fail "unsupported OS $(uname -s); try: go install github.com/$repo/cmd/isekai@latest" ;;
esac
case "$(uname -m)" in
  x86_64|amd64) arch=amd64 ;;
  arm64|aarch64) arch=arm64 ;;
  *) fail "unsupported CPU $(uname -m); try: go install github.com/$repo/cmd/isekai@latest" ;;
esac

if command -v curl >/dev/null 2>&1; then
  get() { curl -fsSL "$1"; }
  download() { curl -fsSL -o "$2" "$1"; }
elif command -v wget >/dev/null 2>&1; then
  get() { wget -qO- "$1"; }
  download() { wget -qO "$2" "$1"; }
else
  fail "need curl or wget"
fi

if [ -z "$version" ]; then
  version=$(get "https://api.github.com/repos/$repo/releases/latest" | sed -n 's/.*"tag_name": *"v\{0,1\}\([^"]*\)".*/\1/p' | head -n 1)
  [ -n "$version" ] || fail "could not find the latest release; set ISEKAI_VERSION"
fi
version=${version#v}

ext=tar.gz
exe=isekai
[ "$os" = windows ] && { ext=zip; exe=isekai.exe; }
name="isekai_${version}_${os}_${arch}.${ext}"
base="${ISEKAI_DOWNLOAD_BASE:-https://github.com/$repo/releases/download/v${version}}"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
echo "downloading $name"
download "$base/$name" "$tmp/$name" || fail "download failed: $base/$name"
download "$base/checksums.txt" "$tmp/checksums.txt" || fail "download failed: checksums.txt"

want=$(grep " $name\$" "$tmp/checksums.txt" | cut -d' ' -f1)
[ -n "$want" ] || fail "$name not listed in checksums.txt"
if command -v sha256sum >/dev/null 2>&1; then got=$(sha256sum "$tmp/$name" | cut -d' ' -f1)
else got=$(shasum -a 256 "$tmp/$name" | cut -d' ' -f1); fi
[ "$want" = "$got" ] || fail "checksum mismatch for $name"

if [ "$ext" = zip ]; then unzip -q "$tmp/$name" -d "$tmp/x"; else mkdir "$tmp/x"; tar -xzf "$tmp/$name" -C "$tmp/x"; fi
mkdir -p "$dir"
mv "$tmp/x/$exe" "$dir/$exe"
chmod +x "$dir/$exe"
echo "installed $dir/$exe ($("$dir/$exe" version))"

case ":$PATH:" in
  *":$dir:"*) ;;
  *) echo "note: $dir is not on PATH; add it, e.g.: export PATH=\"$dir:\$PATH\"" ;;
esac
echo "next: cd <your repo> && isekai init"
