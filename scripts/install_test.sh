#!/usr/bin/env bash
# Wrapper: install.sh integration tests live in Go (internal/update/install_script_test.go).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "${ROOT}"
go test -buildvcs=false -run TestInstallShellScript ./internal/update/
