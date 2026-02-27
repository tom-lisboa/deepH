#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
INSTALL_DIR="${HOME}/.local/bin"
BIN_PATH="${INSTALL_DIR}/deeph"

if ! command -v go >/dev/null 2>&1; then
  echo "error: Go not found. Install Go 1.24+ first."
  exit 1
fi

mkdir -p "${INSTALL_DIR}"

echo "Building deeph..."
go build -o "${BIN_PATH}" "${ROOT_DIR}/cmd/deeph"

echo
echo "Installed: ${BIN_PATH}"
echo
echo "If \`deeph\` is not found, add this to your shell profile:"
echo "  export PATH=\"${INSTALL_DIR}:\$PATH\""
echo
echo "Quick start:"
echo "  mkdir -p ~/deeph-workspace && cd ~/deeph-workspace"
echo "  ${BIN_PATH} quickstart --deepseek"
echo "  export DEEPSEEK_API_KEY=\"sua_chave_real\""
echo "  ${BIN_PATH} run guide \"teste\""
echo
echo "Tip: if you prefer local mock only:"
echo "  ${BIN_PATH} quickstart --provider local_mock --model mock-small"
