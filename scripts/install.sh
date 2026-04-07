#!/usr/bin/env sh

set -eu

REPO="${FJ_REPO:-riadafridishibly/fj}"
BIN_NAME="${FJ_BIN_NAME:-fj}"
INSTALL_DIR="${FJ_INSTALL_DIR:-$HOME/.local/bin}"
VERSION="${FJ_VERSION:-}"

need_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    printf 'error: required command not found: %s\n' "$1" >&2
    exit 1
  fi
}

fetch() {
  url="$1"
  out="$2"

  if command -v curl >/dev/null 2>&1; then
    curl -fsSL -o "$out" "$url"
    return
  fi

  if command -v wget >/dev/null 2>&1; then
    wget -qO "$out" "$url"
    return
  fi

  printf 'error: either curl or wget is required\n' >&2
  exit 1
}

api_get() {
  url="$1"

  if command -v curl >/dev/null 2>&1; then
    if [ -n "${GITHUB_TOKEN:-}" ]; then
      curl -fsSL -H "Authorization: Bearer ${GITHUB_TOKEN}" -H "Accept: application/vnd.github+json" "$url"
    else
      curl -fsSL -H "Accept: application/vnd.github+json" "$url"
    fi
    return
  fi

  if command -v wget >/dev/null 2>&1; then
    if [ -n "${GITHUB_TOKEN:-}" ]; then
      wget -qO- --header="Authorization: Bearer ${GITHUB_TOKEN}" --header="Accept: application/vnd.github+json" "$url"
    else
      wget -qO- --header="Accept: application/vnd.github+json" "$url"
    fi
    return
  fi

  printf 'error: either curl or wget is required\n' >&2
  exit 1
}

resolve_os() {
  case "$(uname -s)" in
    Linux) printf 'linux\n' ;;
    Darwin) printf 'darwin\n' ;;
    *)
      printf 'error: unsupported operating system: %s\n' "$(uname -s)" >&2
      exit 1
      ;;
  esac
}

resolve_arch() {
  case "$(uname -m)" in
    x86_64|amd64) printf 'amd64\n' ;;
    arm64|aarch64) printf 'arm64\n' ;;
    *)
      printf 'error: unsupported architecture: %s\n' "$(uname -m)" >&2
      exit 1
      ;;
  esac
}

resolve_version() {
  if [ -n "$VERSION" ]; then
    printf '%s\n' "$VERSION"
    return
  fi

  api_get "https://api.github.com/repos/${REPO}/releases/latest" \
    | sed -n 's/.*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/p' \
    | head -n 1
}

verify_checksum() {
  archive="$1"
  checksums="$2"

  if command -v sha256sum >/dev/null 2>&1; then
    (cd "$(dirname "$archive")" && sha256sum -c "$checksums" --ignore-missing)
    return
  fi

  if command -v shasum >/dev/null 2>&1; then
    expected="$(grep "  $(basename "$archive")\$" "$checksums" | awk '{print $1}')"
    actual="$(shasum -a 256 "$archive" | awk '{print $1}')"
    if [ "$expected" != "$actual" ]; then
      printf 'error: checksum verification failed for %s\n' "$archive" >&2
      exit 1
    fi
    return
  fi

  printf 'warning: sha256 verification skipped; install sha256sum or shasum to enable it\n' >&2
}

need_cmd tar
need_cmd uname
need_cmd mktemp
need_cmd sed
need_cmd head
need_cmd grep
need_cmd awk
need_cmd mkdir
need_cmd chmod
need_cmd rm

OS="$(resolve_os)"
ARCH="$(resolve_arch)"

printf 'Detected OS: %s, Arch: %s\n' "$OS" "$ARCH"

# Check for existing installation
if [ -x "${INSTALL_DIR}/${BIN_NAME}" ]; then
  CURRENT_VERSION="$("${INSTALL_DIR}/${BIN_NAME}" version 2>/dev/null || printf 'unknown')"
  printf 'Current version: %s\n' "$CURRENT_VERSION"
fi

TAG="$(resolve_version)"

if [ -z "$TAG" ]; then
  printf 'error: unable to resolve a release tag from GitHub Releases\n' >&2
  exit 1
fi

printf 'Installing %s %s to %s\n' "$BIN_NAME" "$TAG" "$INSTALL_DIR"

VERSION_NO_V="${TAG#v}"
ARCHIVE="${BIN_NAME}_${VERSION_NO_V}_${OS}_${ARCH}.tar.gz"
CHECKSUMS="${BIN_NAME}_${VERSION_NO_V}_checksums.txt"
BASE_URL="https://github.com/${REPO}/releases/download/${TAG}"

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT INT TERM

printf 'Downloading %s...\n' "$ARCHIVE"
fetch "${BASE_URL}/${ARCHIVE}" "${TMP_DIR}/${ARCHIVE}"
fetch "${BASE_URL}/${CHECKSUMS}" "${TMP_DIR}/${CHECKSUMS}"

printf 'Verifying checksum...\n'
verify_checksum "${TMP_DIR}/${ARCHIVE}" "${TMP_DIR}/${CHECKSUMS}"

mkdir -p "$INSTALL_DIR"
tar -xzf "${TMP_DIR}/${ARCHIVE}" -C "$TMP_DIR" "$BIN_NAME"
chmod +x "${TMP_DIR}/${BIN_NAME}"
mv "${TMP_DIR}/${BIN_NAME}" "${INSTALL_DIR}/${BIN_NAME}"

printf 'Successfully installed %s %s to %s/%s\n' "$BIN_NAME" "$TAG" "$INSTALL_DIR" "$BIN_NAME"
case ":$PATH:" in
  *":${INSTALL_DIR}:"*) ;;
  *)
    printf 'warning: %s is not on your PATH\n' "$INSTALL_DIR" >&2
    ;;
esac
