#!/bin/sh
set -eu

# ---------------------------------------------------------------
# ghx installer — https://get.rexov.as/ghx/install.sh
#
# Usage:
#   curl -sL https://get.rexov.as/ghx/install.sh | sh
#   curl -sL https://get.rexov.as/ghx/install.sh | sh -s -- --replace-gh
# ---------------------------------------------------------------

CDN="https://get.rexov.as"
INSTALL_DIR="/usr/local/bin"
REPLACE_GH=0

for arg in "$@"; do
  case "$arg" in
    --replace-gh) REPLACE_GH=1 ;;
    *) echo "Unknown option: $arg" >&2; exit 1 ;;
  esac
done

# ---------------------------------------------------------------
# Detect platform
# ---------------------------------------------------------------

detect_os() {
  case "$(uname -s)" in
    Linux)  echo "linux" ;;
    Darwin) echo "darwin" ;;
    *)
      echo "ERROR: Unsupported OS '$(uname -s)'. ghx supports linux and darwin." >&2
      exit 1
      ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64)  echo "amd64" ;;
    arm64|aarch64) echo "arm64" ;;
    *)
      echo "ERROR: Unsupported architecture '$(uname -m)'. ghx supports amd64 and arm64." >&2
      exit 1
      ;;
  esac
}

OS="$(detect_os)"
ARCH="$(detect_arch)"

echo "Detected platform: ${OS}/${ARCH}"

# ---------------------------------------------------------------
# Download binary + checksums
# ---------------------------------------------------------------

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

echo "Downloading ghx..."
curl -sSL -o "${TMP_DIR}/ghx" "${CDN}/ghx/download?os=${OS}&arch=${ARCH}"

echo "Downloading checksums..."
curl -sSL -o "${TMP_DIR}/checksums.txt" "${CDN}/ghx/checksums.txt"

# ---------------------------------------------------------------
# Verify checksum
# ---------------------------------------------------------------

BINARY_NAME="gh_${OS}_${ARCH}"

if command -v sha256sum >/dev/null 2>&1; then
  ACTUAL=$(sha256sum "${TMP_DIR}/ghx" | awk '{print $1}')
elif command -v shasum >/dev/null 2>&1; then
  ACTUAL=$(shasum -a 256 "${TMP_DIR}/ghx" | awk '{print $1}')
else
  echo "WARNING: No sha256sum or shasum found — skipping checksum verification." >&2
  ACTUAL=""
fi

if [ -n "$ACTUAL" ]; then
  EXPECTED=$(grep "${BINARY_NAME}" "${TMP_DIR}/checksums.txt" | awk '{print $1}')
  if [ -z "$EXPECTED" ]; then
    echo "WARNING: No checksum found for ${BINARY_NAME} in checksums.txt — skipping verification." >&2
  elif [ "$ACTUAL" != "$EXPECTED" ]; then
    echo "ERROR: Checksum mismatch!" >&2
    echo "  Expected: ${EXPECTED}" >&2
    echo "  Actual:   ${ACTUAL}" >&2
    exit 1
  else
    echo "Checksum verified."
  fi
fi

# ---------------------------------------------------------------
# Install
# ---------------------------------------------------------------

chmod +x "${TMP_DIR}/ghx"

if [ ! -w "$INSTALL_DIR" ]; then
  echo "Installing to ${INSTALL_DIR} (requires sudo)..."
  sudo cp "${TMP_DIR}/ghx" "${INSTALL_DIR}/ghx"
else
  cp "${TMP_DIR}/ghx" "${INSTALL_DIR}/ghx"
fi

echo "Installed ghx to ${INSTALL_DIR}/ghx"

# ---------------------------------------------------------------
# Replace gh?
# ---------------------------------------------------------------

if [ "$REPLACE_GH" -eq 1 ]; then
  if [ ! -w "$INSTALL_DIR" ]; then
    sudo cp "${INSTALL_DIR}/ghx" "${INSTALL_DIR}/gh"
  else
    cp "${INSTALL_DIR}/ghx" "${INSTALL_DIR}/gh"
  fi
  echo "Also installed as ${INSTALL_DIR}/gh"
elif command -v gh >/dev/null 2>&1; then
  # Only prompt if stdin is a terminal (not piped)
  if [ -t 0 ]; then
    printf "Stock gh detected. Also install ghx as gh? (y/n) "
    read -r REPLY
    if [ "$REPLY" = "y" ] || [ "$REPLY" = "Y" ]; then
      if [ ! -w "$INSTALL_DIR" ]; then
        sudo cp "${INSTALL_DIR}/ghx" "${INSTALL_DIR}/gh"
      else
        cp "${INSTALL_DIR}/ghx" "${INSTALL_DIR}/gh"
      fi
      echo "Also installed as ${INSTALL_DIR}/gh"
    fi
  fi
fi

# ---------------------------------------------------------------
# Done
# ---------------------------------------------------------------

echo ""
"${INSTALL_DIR}/ghx" --version
echo ""
echo "Run 'ghx --version' to verify, or pass --replace-gh to also install as gh."
