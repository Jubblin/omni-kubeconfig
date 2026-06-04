#!/usr/bin/env bash
# CI-parity checks — run inside the dev container (see: make dc-check).
set -euo pipefail

cd "$(dirname "$0")/.."

export PATH="${PATH}:${HOME}/go/bin}"
export GOFLAGS="-buildvcs=false"
git config --global --add safe.directory "$(pwd)" 2>/dev/null || true

echo "==> go test"
make test

echo "==> go vet"
go vet ./...

echo "==> golangci-lint"
make lint

echo "==> goreleaser check"
goreleaser check

echo "==> hadolint"
hadolint Dockerfile

echo "==> build (host)"
make build VERSION=0.0.0-dev

echo "==> build-all (cross-compile)"
make build-all VERSION=0.0.0-dev

echo "==> sbom"
make sbom

echo "All devcontainer checks passed."
