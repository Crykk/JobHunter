#!/bin/sh
# Install jobhunter onto PATH.
# Usage: curl -fsSL https://raw.githubusercontent.com/<org>/jobhunter/main/install.sh | sh
set -eu

REPO="${JOBHUNTER_REPO:-Crykk/JobHunter}"
INSTALL_DIR="${JOBHUNTER_INSTALL_DIR:-${HOME}/.local/bin}"
MODULE="${JOBHUNTER_MODULE:-github.com/Crykk/JobHunter/cmd/jobhunter}"
BIN_NAME="jobhunter"

os=$(uname -s)
arch=$(uname -m)

case "$os" in
  Linux)  goreleaser_os="Linux" ;;
  Darwin) goreleaser_os="Darwin" ;;
  *)
    echo "unsupported OS: $os" >&2
    exit 1
    ;;
esac

case "$arch" in
  x86_64|amd64) goreleaser_arch="x86_64" ;;
  arm64|aarch64) goreleaser_arch="arm64" ;;
  *)
    echo "unsupported arch: $arch" >&2
    exit 1
    ;;
esac

asset="${BIN_NAME}_${goreleaser_os}_${goreleaser_arch}.tar.gz"
api="https://api.github.com/repos/${REPO}/releases/latest"

tmpdir=$(mktemp -d)
trap 'rm -rf "$tmpdir"' EXIT

download_release() {
  json=$(curl -fsSL "$api" 2>/dev/null || true)
  if [ -z "$json" ]; then
    return 1
  fi
  url=$(printf '%s' "$json" | sed -n "s/.*\"browser_download_url\": \"\\([^\"]*${asset}\\)\".*/\\1/p" | head -n 1)
  if [ -z "$url" ]; then
    return 1
  fi
  echo "Downloading ${url}"
  curl -fL --progress-bar -o "${tmpdir}/${asset}" "$url"
  tar -xzf "${tmpdir}/${asset}" -C "$tmpdir"
}

go_fallback() {
  if ! command -v go >/dev/null 2>&1; then
    echo "No GitHub Release found for ${REPO} and Go is not on PATH." >&2
    echo "Create a release, set JOBHUNTER_REPO=owner/name, or install Go and retry." >&2
    exit 1
  fi
  echo "No GitHub Release asset ${asset}; falling back to go install ${MODULE}@latest"
  GOBIN="$INSTALL_DIR" go install "${MODULE}@latest"
}

mkdir -p "$INSTALL_DIR"

if download_release; then
  src=$(find "$tmpdir" -type f -name "$BIN_NAME" | head -n 1)
  if [ -z "$src" ]; then
    echo "archive did not contain ${BIN_NAME}" >&2
    exit 1
  fi
  install -m 0755 "$src" "${INSTALL_DIR}/${BIN_NAME}"
  data_dir="${XDG_DATA_HOME:-$HOME/.local/share}/jobhunter"
  mkdir -p "$data_dir"
  example=$(find "$tmpdir" -type f -name "profile.example.txt" | head -n 1)
  if [ -n "$example" ] && [ ! -f "${data_dir}/profile.txt" ]; then
    cp "$example" "${data_dir}/profile.txt"
    echo "Created ${data_dir}/profile.txt — edit it in JobHunter Settings"
  fi
else
  go_fallback
fi

echo "Installed ${INSTALL_DIR}/${BIN_NAME}"
case ":$PATH:" in
  *":${INSTALL_DIR}:"*) ;;
  *)
    echo "Add ${INSTALL_DIR} to PATH, for example:"
    echo "  export PATH=\"${INSTALL_DIR}:\$PATH\""
    ;;
esac
echo "Then run: jobhunter   (same-tab UI). Scripts: jobhunter search -q python -a Helsinki"
