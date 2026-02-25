#!/usr/bin/env bash
set -euo pipefail

REPO="${WTX_REPO:-aixolotls/wtx}"
BINARY_NAME="wtx"
VERSION="${WTX_VERSION:-latest}"
INSTALL_DIR="${WTX_INSTALL_DIR:-${INSTALL_DIR:-$HOME/.local/bin}}"

log() {
  printf '%s\n' "$*"
}

fail() {
  printf 'Error: %s\n' "$*" >&2
  exit 1
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || fail "missing required command: $1"
}

download() {
  local url="$1"
  local out="$2"
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$url" -o "$out"
    return
  fi
  if command -v wget >/dev/null 2>&1; then
    wget -qO "$out" "$url"
    return
  fi
  fail "curl or wget is required"
}

detect_os() {
  local os
  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  case "$os" in
    linux|darwin)
      printf '%s\n' "$os"
      ;;
    *)
      fail "unsupported OS: $os (supported: linux, darwin)"
      ;;
  esac
}

detect_arch() {
  local arch
  arch="$(uname -m)"
  case "$arch" in
    x86_64|amd64)
      printf 'amd64\n'
      ;;
    arm64|aarch64)
      printf 'arm64\n'
      ;;
    *)
      fail "unsupported architecture: $arch (supported: amd64, arm64)"
      ;;
  esac
}

resolve_asset_urls() {
  local os="$1"
  local arch="$2"
  local archive="wtx_${os}_${arch}.tar.gz"
  if [ "$VERSION" = "latest" ]; then
    printf 'https://github.com/%s/releases/latest/download/%s\n' "$REPO" "$archive"
    printf 'https://github.com/%s/releases/latest/download/checksums.txt\n' "$REPO"
    return
  fi
  printf 'https://github.com/%s/releases/download/%s/%s\n' "$REPO" "$VERSION" "$archive"
  printf 'https://github.com/%s/releases/download/%s/checksums.txt\n' "$REPO" "$VERSION"
}

verify_checksum() {
  local archive_path="$1"
  local checksums_path="$2"
  local archive_name
  archive_name="$(basename "$archive_path")"

  if ! grep -q "  ${archive_name}\$" "$checksums_path"; then
    log "Checksum entry for ${archive_name} not found; skipping verification"
    return
  fi

  if command -v sha256sum >/dev/null 2>&1; then
    (cd "$(dirname "$checksums_path")" && grep "  ${archive_name}\$" "$checksums_path" | sha256sum -c -)
    return
  fi
  if command -v shasum >/dev/null 2>&1; then
    (cd "$(dirname "$checksums_path")" && grep "  ${archive_name}\$" "$checksums_path" | shasum -a 256 -c -)
    return
  fi

  log "sha256sum/shasum not found; skipping checksum verification"
}

main() {
  require_cmd uname
  require_cmd tar
  require_cmd mktemp

  local os arch
  os="$(detect_os)"
  arch="$(detect_arch)"

  local urls release_url checksums_url
  urls="$(resolve_asset_urls "$os" "$arch")"
  release_url="$(printf '%s\n' "$urls" | sed -n '1p')"
  checksums_url="$(printf '%s\n' "$urls" | sed -n '2p')"

  local tmp_dir archive_path checksums_path extracted_path target_path
  tmp_dir="$(mktemp -d)"
  archive_path="${tmp_dir}/wtx.tar.gz"
  checksums_path="${tmp_dir}/checksums.txt"
  extracted_path="${tmp_dir}/${BINARY_NAME}"
  target_path="${INSTALL_DIR}/${BINARY_NAME}"
  trap 'rm -rf "${tmp_dir}"' EXIT

  log "Downloading ${release_url}"
  download "$release_url" "$archive_path"

  if download "$checksums_url" "$checksums_path"; then
    verify_checksum "$archive_path" "$checksums_path"
  else
    log "Could not download checksums.txt; continuing without checksum verification"
  fi

  tar -xzf "$archive_path" -C "$tmp_dir"
  [ -f "$extracted_path" ] || fail "archive did not contain ${BINARY_NAME}"

  mkdir -p "$INSTALL_DIR"
  install -m 0755 "$extracted_path" "$target_path"

  log "Installed ${BINARY_NAME} to ${target_path}"
  case ":$PATH:" in
    *":${INSTALL_DIR}:"*) ;;
    *)
      log "Add this to your shell profile:"
      log "  export PATH=\"${INSTALL_DIR}:\$PATH\""
      ;;
  esac

  "${target_path}" --version || true
}

main "$@"
