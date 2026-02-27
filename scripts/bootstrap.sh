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
echo "  deeph init"
echo "  cp ${ROOT_DIR}/examples/agents/guide.yaml agents/guide.yaml"
echo "  deeph skill add echo"
echo "  deeph validate"
echo "  deeph run guide \"teste\""
echo
echo "Optional (DeepSeek real):"
echo "  deeph provider add deepseek --set-default"
echo "  export DEEPSEEK_API_KEY=\"sua_chave_aqui\""
echo "  deeph run guide \"teste com DeepSeek\""
