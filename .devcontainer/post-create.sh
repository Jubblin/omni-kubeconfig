#!/usr/bin/env bash
set -euo pipefail

go mod download

# Match CI (.github/workflows/ci.yml): golangci-lint v2.1.6
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.1.6

# Hadolint + GoReleaser binaries (avoid go install; goreleaser v2.16+ needs Go 1.26+)
arch="$(uname -m)"
case "${arch}" in
  x86_64|amd64)
    hadolint_arch=Linux-x86_64
    goreleaser_arch=Linux_x86_64
    ;;
  aarch64|arm64)
    hadolint_arch=Linux-arm64
    goreleaser_arch=Linux_arm64
    ;;
  *)
    echo "unsupported arch: ${arch}" >&2
    exit 1
    ;;
esac

goreleaser_version=v2.16.0
curl -sSfL "https://github.com/goreleaser/goreleaser/releases/download/${goreleaser_version}/goreleaser_${goreleaser_arch}.tar.gz" \
  | tar -xz -C /tmp goreleaser
sudo install -m 0755 /tmp/goreleaser /usr/local/bin/goreleaser
rm -f /tmp/goreleaser

curl -sSfL -o /tmp/hadolint "https://github.com/hadolint/hadolint/releases/download/v2.12.0/hadolint-${hadolint_arch}"
sudo install -m 0755 /tmp/hadolint /usr/local/bin/hadolint
rm -f /tmp/hadolint

echo "devcontainer tools ready: golangci-lint, goreleaser ${goreleaser_version}, hadolint"
