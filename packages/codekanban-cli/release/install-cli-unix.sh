#!/usr/bin/env sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)

npm install -g --no-fund --no-audit \
  "$SCRIPT_DIR/npm/codekanban-cli-__CLI_VERSION__.tgz"

echo "CLI install complete. Run ./install-skills-unix.sh as the real Codex user next."
