#!/usr/bin/env bash
# Install omni-kubeconfig from GitHub releases (checksum-verified).
set -euo pipefail

REPO="${GITHUB_REPO:-Jubblin/omni-kubeconfig}"
API_BASE="${GITHUB_API_URL:-https://api.github.com}"
DOWNLOAD_BASE="${GITHUB_DOWNLOAD_URL:-https://github.com/${REPO}/releases/download}"
LATEST_RELEASE_URL="${GITHUB_LATEST_URL:-https://github.com/${REPO}/releases/latest}"
PROJECT_NAME="omni-kubeconfig"

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "error: required command not found: $1" >&2
    exit 1
  }
}

sha256_file() {
  local file="$1"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$file" | awk '{print $1}'
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$file" | awk '{print $1}'
  else
    echo "error: sha256sum or shasum required" >&2
    exit 1
  fi
}

detect_platform() {
  local os arch
  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  arch="$(uname -m)"
  case "${os}" in
    linux) os="linux" ;;
    darwin) os="darwin" ;;
    *) echo "error: unsupported OS ${os}" >&2; exit 1 ;;
  esac
  case "${arch}" in
    x86_64|amd64) arch="amd64" ;;
    aarch64|arm64) arch="arm64" ;;
    *) echo "error: unsupported architecture ${arch}" >&2; exit 1 ;;
  esac
  ASSET_NAME="${PROJECT_NAME}-${os}-${arch}"
  INSTALL_NAME="${PROJECT_NAME}"
  GOOS="${os}"
}

default_install_dir() {
  if [[ -n "${INSTALL_DIR:-}" ]]; then
    echo "${INSTALL_DIR}"
    return
  fi
  echo "${HOME}/.local/bin"
}

tag_from_latest_redirect() {
  curl -fsSI "${LATEST_RELEASE_URL}" 2>/dev/null \
    | awk -F'/tag/' 'tolower($0) ~ /^location:/ { print $2; exit }' \
    | tr -d '\r\n'
}

tag_from_api_latest() {
  curl -fsSL \
    -H "Accept: application/vnd.github+json" \
    "${API_BASE}/repos/${REPO}/releases/latest" 2>/dev/null \
    | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' \
    | head -1
}

tag_from_api_list() {
  curl -fsSL \
    -H "Accept: application/vnd.github+json" \
    "${API_BASE}/repos/${REPO}/releases?per_page=30" 2>/dev/null \
    | awk '
      /"tag_name"/ {
        line = $0
        sub(/.*"tag_name"[[:space:]]*:[[:space:]]*"/, "", line)
        sub(/".*/, "", line)
        tag = line
      }
      /"prerelease"[[:space:]]*:[[:space:]]*false/ && tag != "" {
        print tag
        exit
      }
    '
}

resolve_version() {
  if [[ -n "${VERSION:-}" ]]; then
    if [[ "${VERSION}" != v* ]]; then
      VERSION="v${VERSION}"
    fi
    echo "${VERSION}"
    return
  fi
  need_cmd curl
  local tag=""
  tag="$(tag_from_latest_redirect)"
  if [[ -z "${tag}" ]]; then
    tag="$(tag_from_api_latest)"
  fi
  if [[ -z "${tag}" ]]; then
    tag="$(tag_from_api_list)"
  fi
  if [[ -z "${tag}" ]]; then
    echo "error: could not resolve latest release tag (set VERSION=vX.Y.Z or use releases/latest/download/install.sh)" >&2
    exit 1
  fi
  echo "${tag}"
}

download() {
  local url="$1" dest="$2"
  curl -fsSL "${url}" -o "${dest}"
}

verify_checksum() {
  local asset="$1" sums="$2" tmp="$3"
  if [[ "${SKIP_SHA256:-}" == "1" ]]; then
    return 0
  fi
  local expected got
  expected="$(grep " ${asset}\$" "${sums}" | awk '{print $1}' | head -1)"
  if [[ -z "${expected}" ]]; then
    expected="$(grep "${asset}\$" "${sums}" | awk '{print $1}' | head -1)"
  fi
  if [[ -z "${expected}" ]]; then
    echo "error: checksum for ${asset} not found" >&2
    exit 1
  fi
  got="$(sha256_file "${tmp}")"
  if [[ "${got}" != "${expected}" ]]; then
    echo "error: checksum mismatch for ${asset}" >&2
    exit 1
  fi
}

main() {
  need_cmd curl
  detect_platform
  TAG="$(resolve_version)"
  INSTALL_DIR="$(default_install_dir)"
  mkdir -p "${INSTALL_DIR}"

  tmpdir="$(mktemp -d)"
  cleanup() { rm -rf "${tmpdir}"; }
  trap cleanup EXIT

  sums="${tmpdir}/sha256sum.txt"
  bin="${tmpdir}/${ASSET_NAME}"
  download "${DOWNLOAD_BASE}/${TAG}/sha256sum.txt" "${sums}"
  download "${DOWNLOAD_BASE}/${TAG}/${ASSET_NAME}" "${bin}"
  verify_checksum "${ASSET_NAME}" "${sums}" "${bin}"

  target="${INSTALL_DIR}/${INSTALL_NAME}"
  install -m 0755 "${bin}" "${target}"

  echo "Installed omni-kubeconfig ${TAG} to ${target}"
  case ":${PATH}:" in
    *":${INSTALL_DIR}:"*) ;;
    *) echo "Add to PATH: export PATH=\"${INSTALL_DIR}:\$PATH\"" ;;
  esac
}

main "$@"
