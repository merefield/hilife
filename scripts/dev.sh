#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
task_go=go
if [[ -x .tools/go/bin/go ]]; then task_go=.tools/go/bin/go; fi
"$task_go" run . web &
task_server_pid=$!
trap 'kill "$task_server_pid" 2>/dev/null || true' EXIT INT TERM
cd web
pnpm dev
