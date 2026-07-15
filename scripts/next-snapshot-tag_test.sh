#!/usr/bin/env bash
# Tests for scripts/next-snapshot-tag.sh
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
SCRIPT="${ROOT}/scripts/next-snapshot-tag.sh"
chmod +x "${SCRIPT}"

tmpdir="$(mktemp -d)"
cleanup() { rm -rf "${tmpdir}"; }
trap cleanup EXIT

git -C "${tmpdir}" init -q
git -C "${tmpdir}" config user.email "test@example.com"
git -C "${tmpdir}" config user.name "test"
git -C "${tmpdir}" commit --allow-empty -qm init >/dev/null

fail=0
assert_eq() {
  local want="$1" got="$2" msg="$3"
  if [[ "${want}" != "${got}" ]]; then
    echo "FAIL: ${msg}: want=${want} got=${got}" >&2
    fail=1
  else
    echo "ok: ${msg}"
  fi
}

cd "${tmpdir}"

got="$("${SCRIPT}" v0.2.0)"
assert_eq "v0.2.0-snapshot.1" "${got}" "first snapshot when none exist"

git tag v0.2.0-snapshot
got="$("${SCRIPT}" v0.2.0)"
assert_eq "v0.2.0-snapshot.1" "${got}" "bare -snapshot does not bump past .1"

git tag v0.2.0-snapshot.1
got="$("${SCRIPT}" v0.2.0)"
assert_eq "v0.2.0-snapshot.2" "${got}" "increments after .1"

git tag v0.2.0-snapshot.3
got="$("${SCRIPT}" v0.2.0)"
assert_eq "v0.2.0-snapshot.4" "${got}" "increments from highest existing N"

got="$("${SCRIPT}" v0.1.0)"
assert_eq "v0.1.0-snapshot.1" "${got}" "independent semver line starts at .1"

if ! "${SCRIPT}" not-a-tag >/dev/null 2>&1; then
  echo "ok: rejects non-semver"
else
  echo "FAIL: should reject non-semver" >&2
  fail=1
fi

exit "${fail}"
