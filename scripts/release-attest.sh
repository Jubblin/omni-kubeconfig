#!/usr/bin/env bash
# Sign and attach release supply-chain artifacts after image promote.
# Usage: release-attest.sh <image@digest> <release-tag>
#
# Expects on disk:
#   dist/sbom.cyclonedx.json          (module SBOM from GoReleaser)
#   dist/sbom-image.cyclonedx.json    (container SBOM from Trivy)
# Requires: cosign, gh; GH_TOKEN for release upload.

set -euo pipefail

if [[ $# -ne 2 ]]; then
  echo "usage: $0 <image@digest> <release-tag>" >&2
  exit 2
fi

image_ref="$1"
release_tag="$2"

export COSIGN_EXPERIMENTAL=1

test -f dist/sbom.cyclonedx.json
test -s dist/sbom-image.cyclonedx.json

cosign sign-blob --yes dist/sbom.cyclonedx.json \
  --bundle dist/sbom.cyclonedx.json.bundle

gh release upload "${release_tag}" \
  dist/sbom.cyclonedx.json.bundle \
  --clobber

cosign sign --yes "${image_ref}"

cosign attach sbom --sbom dist/sbom-image.cyclonedx.json \
  "${image_ref}"

cosign sign-blob --yes dist/sbom-image.cyclonedx.json \
  --bundle dist/sbom-image.cyclonedx.json.bundle

echo "Attested ${image_ref} for ${release_tag}"
