#!/usr/bin/env sh
set -eu

cat <<'EOF'
Use the Unix install in two separate steps:

  1. ./install-cli-unix.sh
  2. ./install-skills-unix.sh

Why split it?
- Global npm installation can require sudo or a privileged prefix.
- Skills must land in the real Codex user's ~/.codex/skills directory.
- Running a combined sudo installer could put skills under root's ~/.codex/skills, which is the wrong account.

After both steps finish, restart Codex.
EOF
