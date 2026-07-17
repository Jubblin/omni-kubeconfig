#!/usr/bin/env bash
# Smoke tests for scripts/release-attest.sh (arg parsing only; no cosign/network).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
SCRIPT="${ROOT}/scripts/release-attest.sh"
chmod +x "${SCRIPT}"

fail=0
if "${SCRIPT}" >/dev/null 2>&1; then
  echo "FAIL: expected usage error with no args" >&2
  fail=1
else
  echo "ok: rejects no args"
fi

if "${SCRIPT}" only-one >/dev/null 2>&1; then
  echo "FAIL: expected usage error with one arg" >&2
  fail=1
else
  echo "ok: rejects one arg"
fi

exit "${fail}"
