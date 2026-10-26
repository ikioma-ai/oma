#!/bin/sh
set -eu

REPO='ikioma-ai/oma'
INSTALL_DIR="${OMA_INSTALL_DIR:-/usr/local/bin}"
INSTALL_REF='{{ .InstallRef }}'
RELEASE_JSON=''
OMA_BIN=''

main() {
  os=$(detect_os)
  arch=$(detect_arch)
  ensure_docker "$os"
  fetch_latest_release
  install_oma "$arch"

  if [ "${OMA_INTERACTIVE:-}" = "false" ]; then
    echo "Oma is installed. Run 'oma' to get started."
    return
  fi

  run_oma
}

detect_os() {
  case "$(uname -s)" in
  Linux*) echo "linux" ;;
  Darwin*) echo "darwin" ;;
  *)
    echo "Unsupported OS: $(uname -s)" >&2
    exit 1
    ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
  x86_64) echo "amd64" ;;
  aarch64) echo "arm64" ;;
  arm64) echo "arm64" ;;
  *)
    echo "Unsupported architecture: $(uname -m)" >&2
    exit 1
    ;;
  esac
}

ensure_docker() {
  os="$1"

  if docker info >/dev/null 2>&1; then
    DOCKER_MODE=normal
    return
  fi

  if command -v docker >/dev/null 2>&1; then
    if [ "$os" = "darwin" ]; then
      echo "Docker Desktop is installed but not running."
      echo "Please start Docker Desktop and try again."
      exit 1
    fi
    DOCKER_MODE=sudo
    return
  fi

  if [ "$os" = "darwin" ]; then
    echo "Docker is required. Install Docker Desktop from"
    echo "https://www.docker.com/products/docker-desktop"
    exit 1
  fi

  echo "Installing Docker..."
  if is_root; then
    sh -c "$(curl -fsSL https://get.docker.com)" >/dev/null 2>&1
  else
    curl -fsSL https://get.docker.com | sudo sh >/dev/null 2>&1
  fi

  if ! is_root; then
    sudo usermod -aG docker "$USER"
  fi

  DOCKER_MODE=sg
}

install_oma() {
  arch="$1"

  if command -v oma >/dev/null 2>&1; then
    OMA_BIN=$(command -v oma)
    echo "Oma is already installed at ${OMA_BIN}"
    return
  fi

  version=$(latest_version)
  echo "Installing Oma ${version}..."

  binary="oma-${os}-${arch}"
  asset_url=$(get_asset_url "$binary")

  if [ -z "$asset_url" ]; then
    echo "Could not find release asset: ${binary}" >&2
    exit 1
  fi

  checksum_url=$(get_asset_url "checksums.txt")
  if [ -z "$checksum_url" ]; then
    echo "Release does not publish checksums.txt" >&2
    exit 1
  fi

  checksum_file=$(mktemp)
  download "$checksum_url" "$checksum_file" "application/octet-stream"

  if ! expected_checksum=$(checksum_for_asset "$checksum_file" "$binary"); then
    rm -f "$checksum_file"
    echo "Could not find a valid checksum for ${binary}" >&2
    exit 1
  fi
  rm -f "$checksum_file"

  tmpfile=$(mktemp)
  download "$asset_url" "$tmpfile" "application/octet-stream"

  if ! verify_sha256 "$tmpfile" "$expected_checksum"; then
    rm -f "$tmpfile"
    echo "Checksum verification failed for ${binary}" >&2
    exit 1
  fi

  if is_root; then
    install -m 755 "$tmpfile" "${INSTALL_DIR}/oma"
  else
    sudo install -m 755 "$tmpfile" "${INSTALL_DIR}/oma"
  fi
  rm -f "$tmpfile"

  OMA_BIN="${INSTALL_DIR}/oma"
  echo "Installed Oma to ${OMA_BIN}"

  echo "Installing background service..."
  if is_root; then
    "${OMA_BIN}" background install
  else
    sudo "${OMA_BIN}" background install
  fi
}

run_oma() {
  install_flag=""
  if [ -n "$INSTALL_REF" ]; then
    install_flag="--install=${INSTALL_REF}"
  fi

  case "$DOCKER_MODE" in
  normal)
    exec "${OMA_BIN}" ${install_flag} </dev/tty
    ;;
  sudo)
    exec sudo "${OMA_BIN}" ${install_flag} </dev/tty
    ;;
  sg)
    sg docker -c "${OMA_BIN} ${install_flag}" </dev/tty
    ;;
  esac
}

# Private

fetch_latest_release() {
  RELEASE_JSON=$(download "https://api.github.com/repos/${REPO}/releases/latest" -)
}

latest_version() {
  echo "$RELEASE_JSON" | awk '/"tag_name"/ { gsub(/.*"tag_name": *"/, ""); gsub(/".*/, ""); print; exit }'
}

get_asset_url() {
  binary="$1"
  echo "$RELEASE_JSON" | awk -v binary="$binary" '
        /"url":.*api\.github\.com.*assets/ {
            u = $0; gsub(/.*"url": *"/, "", u); gsub(/".*/, "", u)
        }
        /"name":/ && index($0, "\"" binary "\"") { print u; exit }
    '
}

checksum_for_asset() {
  checksum_file="$1"
  binary="$2"

  awk -v binary="$binary" '
    {
      if (NF != 2) { bad = 1; next }
      digest = $1
      asset = $2
      sub(/^\*/, "", asset)
      if (length(digest) != 64 || digest !~ /^[0-9a-f]+$/) { bad = 1; next }
      if (asset == binary) { count++; expected = digest }
    }
    END {
      if (bad || count != 1) exit 1
      print expected
    }
  ' "$checksum_file"
}

verify_sha256() {
  file="$1"
  expected="$2"

  if command -v sha256sum >/dev/null 2>&1; then
    actual=$(sha256sum "$file" | awk '{ print $1 }')
  elif command -v shasum >/dev/null 2>&1; then
    actual=$(shasum -a 256 "$file" | awk '{ print $1 }')
  else
    echo "sha256sum or shasum is required to verify downloads" >&2
    return 1
  fi

  [ "$actual" = "$expected" ]
}

download() {
  url="$1"
  output="$2"
  accept="${3:-}"

  if command -v curl >/dev/null 2>&1; then
    set -- -fsSL -o "$output"
    [ -n "${GITHUB_TOKEN:-}" ] && set -- "$@" -H "Authorization: token ${GITHUB_TOKEN}"
    [ -n "$accept" ] && set -- "$@" -H "Accept: $accept"
    curl "$@" "$url"
  elif command -v wget >/dev/null 2>&1; then
    set -- -q -O "$output"
    [ -n "${GITHUB_TOKEN:-}" ] && set -- "$@" --header="Authorization: token ${GITHUB_TOKEN}"
    [ -n "$accept" ] && set -- "$@" --header="Accept: $accept"
    wget "$@" "$url"
  else
    echo "curl or wget is required" >&2
    exit 1
  fi
}

is_root() {
  [ "$(id -u)" -eq 0 ]
}

main
