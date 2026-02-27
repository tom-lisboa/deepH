#!/usr/bin/env bash
set -euo pipefail

OWNER="${DEEPH_GITHUB_OWNER:-tom-lisboa}"
REPO="${DEEPH_GITHUB_REPO:-deepH}"
VERSION="${1:-latest}"
INSTALL_DIR="${DEEPH_INSTALL_DIR:-$HOME/.local/bin}"
BIN_PATH="${INSTALL_DIR}/deeph"

need_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "error: required command not found: $1"
    exit 1
  fi
}

need_cmd uname
if command -v curl >/dev/null 2>&1; then
  FETCH_BIN="curl"
elif command -v wget >/dev/null 2>&1; then
  FETCH_BIN="wget"
else
  echo "error: curl or wget is required"
  exit 1
fi

OS="$(uname -s)"
ARCH="$(uname -m)"

case "$OS" in
  Darwin)
    case "$ARCH" in
      arm64|aarch64) ASSET="deeph-darwin-arm64" ;;
      x86_64|amd64) ASSET="deeph-darwin-amd64" ;;
      *) echo "error: unsupported macOS arch: $ARCH"; exit 1 ;;
    esac
    ;;
  Linux)
    case "$ARCH" in
      arm64|aarch64) ASSET="deeph-linux-arm64" ;;
      x86_64|amd64) ASSET="deeph-linux-amd64" ;;
      *) echo "error: unsupported Linux arch: $ARCH"; exit 1 ;;
    esac
    ;;
  *)
    echo "error: unsupported OS: $OS"
    exit 1
    ;;
esac

if [[ "$VERSION" == "latest" ]]; then
  BASE_URL="https://github.com/${OWNER}/${REPO}/releases/latest/download"
else
  BASE_URL="https://github.com/${OWNER}/${REPO}/releases/download/${VERSION}"
fi

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

ASSET_URL="${BASE_URL}/${ASSET}"
CHECKSUMS_URL="${BASE_URL}/checksums.txt"
ASSET_TMP="${TMP_DIR}/${ASSET}"
CHECKSUMS_TMP="${TMP_DIR}/checksums.txt"

fetch() {
  local url="$1"
  local out="$2"
  if [[ "$FETCH_BIN" == "curl" ]]; then
    curl -fsSL "$url" -o "$out"
  else
    wget -qO "$out" "$url"
  fi
}

echo "Downloading ${ASSET_URL}"
fetch "$ASSET_URL" "$ASSET_TMP"

if fetch "$CHECKSUMS_URL" "$CHECKSUMS_TMP"; then
  if command -v sha256sum >/dev/null 2>&1; then
    EXPECTED="$(awk -v a="$ASSET" '$2==a {print $1}' "$CHECKSUMS_TMP")"
    ACTUAL="$(sha256sum "$ASSET_TMP" | awk '{print $1}')"
  elif command -v shasum >/dev/null 2>&1; then
    EXPECTED="$(awk -v a="$ASSET" '$2==a {print $1}' "$CHECKSUMS_TMP")"
    ACTUAL="$(shasum -a 256 "$ASSET_TMP" | awk '{print $1}')"
  else
    EXPECTED=""
    ACTUAL=""
  fi

  if [[ -n "${EXPECTED:-}" && -n "${ACTUAL:-}" ]]; then
    if [[ "$EXPECTED" != "$ACTUAL" ]]; then
      echo "error: checksum mismatch for ${ASSET}"
      exit 1
    fi
  fi
fi

mkdir -p "$INSTALL_DIR"
cp "$ASSET_TMP" "$BIN_PATH"
chmod +x "$BIN_PATH"

echo
echo "Installed deeph at: $BIN_PATH"
echo "Run: deeph"
if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
  echo
  echo "PATH note: add this to your shell profile:"
  echo "  export PATH=\"$INSTALL_DIR:\$PATH\""
fi
