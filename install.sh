#!/bin/sh
# Installs the latest quipu release into ~/.local/bin (no root needed).
#
#   curl -fsSL https://raw.githubusercontent.com/axvilab/quipu/main/install.sh | sh
#
# Environment:
#   QUIPU_VERSION      a tag such as v0.1.0 (default: latest release)
#   QUIPU_INSTALL_DIR  where to put quipu and quipu-app (default: ~/.local/bin); on macOS the
#                      Quipu app goes to ~/Applications
#   QUIPU_BASE_URL     download location, for mirrors and tests
set -eu

repo="axvilab/quipu"
install_dir="${QUIPU_INSTALL_DIR:-$HOME/.local/bin}"
version="${QUIPU_VERSION:-latest}"

say() { printf 'quipu: %s\n' "$*"; }
fail() { printf 'quipu: %s\n' "$*" >&2; exit 1; }

case "$(uname -s)" in
  Linux) os=linux ;;
  Darwin) os=darwin ;;
  *) fail "unsupported system $(uname -s); quipu runs on Linux and macOS (on Windows use WSL)" ;;
esac
case "$(uname -m)" in
  x86_64 | amd64) arch=amd64 ;;
  aarch64 | arm64) arch=arm64 ;;
  *) fail "unsupported CPU $(uname -m); quipu is built for x86_64 and arm64" ;;
esac
if [ "$os" = linux ] && ldd --version 2>&1 | grep -qi musl; then
  fail "musl-based systems such as Alpine are not supported (quipu needs glibc)"
fi

if [ -n "${QUIPU_BASE_URL:-}" ]; then
  base="$QUIPU_BASE_URL"
elif [ "$version" = latest ]; then
  base="https://github.com/$repo/releases/latest/download"
else
  base="https://github.com/$repo/releases/download/$version"
fi
asset="quipu_${os}_${arch}.tar.gz"

if command -v curl >/dev/null 2>&1; then
  fetch() { curl -fsSL -o "$2" "$1"; }
elif command -v wget >/dev/null 2>&1; then
  fetch() { wget -q -O "$2" "$1"; }
else
  fail "need curl or wget"
fi
if command -v sha256sum >/dev/null 2>&1; then
  sha256() { sha256sum "$1" | cut -d' ' -f1; }
else
  sha256() { shasum -a 256 "$1" | cut -d' ' -f1; }
fi

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

say "downloading $asset ($version)"
fetch "$base/$asset" "$tmp/$asset" || fail "download failed: $base/$asset"
fetch "$base/checksums.txt" "$tmp/checksums.txt" || fail "download failed: $base/checksums.txt"

want="$(grep " $asset\$" "$tmp/checksums.txt" | cut -d' ' -f1)"
got="$(sha256 "$tmp/$asset")"
[ -n "$want" ] || fail "no checksum for $asset in checksums.txt"
[ "$want" = "$got" ] || fail "checksum mismatch for $asset: the download is corrupt or was tampered with"

mkdir "$tmp/x"
tar xzf "$tmp/$asset" -C "$tmp/x"
mkdir -p "$install_dir"
mv "$tmp/x/quipu" "$install_dir/quipu"
chmod 755 "$install_dir/quipu"
say "installed $("$install_dir/quipu" version) to $install_dir/quipu"

# The quipu window (ADR 0007). Without it, quipu opens in the browser.
if [ -f "$tmp/x/quipu-app" ]; then
  mv "$tmp/x/quipu-app" "$install_dir/quipu-app"
  chmod 755 "$install_dir/quipu-app"
  if ! "$install_dir/quipu-app" -version >/dev/null 2>&1; then
    say "the quipu window needs WebKitGTK 4.1; until it is installed, quipu opens in the browser"
    say "  Debian/Ubuntu: sudo apt install libwebkit2gtk-4.1-0    Fedora: sudo dnf install webkit2gtk4.1"
  fi
fi
if [ -d "$tmp/x/Quipu.app" ]; then
  apps="$HOME/Applications"
  mkdir -p "$apps"
  rm -rf "$apps/Quipu.app"
  mv "$tmp/x/Quipu.app" "$apps/Quipu.app"
  say "installed the Quipu app to $apps/Quipu.app"
fi

case ":$PATH:" in
  *":$install_dir:"*) ;;
  *) say "$install_dir is not in your PATH; add it, e.g.: echo 'export PATH=\"$install_dir:\$PATH\"' >> ~/.profile" ;;
esac
say "next: run 'quipu install' to start it at login, then 'quipu ui' to open it"
