#!/usr/bin/env bash
# Print the next snapshot tag for a semver release line.
# Usage: next-snapshot-tag.sh <latest-semver-tag>
# Example: next-snapshot-tag.sh v0.2.0  ->  v0.2.0-snapshot.1
#
# Scans existing tags matching:
#   <base>-snapshot
#   <base>-snapshot.<N>
# and returns <base>-snapshot.<next>.

set -euo pipefail

if [[ $# -ne 1 ]]; then
  echo "usage: $0 <latest-semver-tag>" >&2
  exit 2
fi

latest="$1"
if [[ ! "${latest}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "error: expected semver tag like v0.2.0, got ${latest}" >&2
  exit 2
fi

base="${latest}-snapshot"
next=1

while IFS= read -r tag; do
  [[ -z "${tag}" ]] && continue
  if [[ "${tag}" =~ ^${base}\.([0-9]+)$ ]]; then
    n="${BASH_REMATCH[1]}"
    if (( n >= next )); then
      next=$((n + 1))
    fi
  fi
done < <(git tag -l "${base}" "${base}.*")

printf '%s.%s\n' "${base}" "${next}"
