#!/usr/bin/env sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
CODEX_HOME_DIR=${CODEX_HOME:-$HOME/.codex}
SKILLS_DIR="$CODEX_HOME_DIR/skills"

mkdir -p "$SKILLS_DIR"
cp -R "$SCRIPT_DIR/skills/." "$SKILLS_DIR/"

echo "Skills installed into $SKILLS_DIR"
echo "Restart Codex to discover the new skills."
