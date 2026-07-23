#!/bin/sh
# Share2Us GUI installer for Linux.
#
#   curl -fsSL https://raw.githubusercontent.com/share2us/gui/main/scripts/install.sh | sh
#
# Always downloads the latest release binary for this OS/arch from GitHub Releases
# (the hosted mirror on share2.us is a fallback), verifies its .crc32 sidecar,
# installs it, and registers the file-manager right-click integration.
set -eu

repo="${SHARE2US_GUI_REPO:-share2us/gui}"
version="${SHARE2US_GUI_VERSION:-latest}"
base_url="${SHARE2US_GUI_BASE_URL:-https://share2.us}"
install_dir="${SHARE2US_GUI_INSTALL_DIR:-$HOME/.local/bin}"
binary_name="share2us-gui"

log()  { printf '%s\n' "$*"; }
fail() { printf 'share2us-gui install: %s\n' "$*" >&2; exit 1; }
need() { command -v "$1" >/dev/null 2>&1 || fail "missing required command: $1"; }

detect_os() {
  case "$(uname -s)" in
    Linux) printf linux ;;
    *) fail "install.sh is for Linux; on macOS use the app bundle (coming soon)" ;;
  esac
}
detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64) printf amd64 ;;
    aarch64|arm64) printf arm64 ;;
    *) fail "unsupported architecture: $(uname -m)" ;;
  esac
}

download_to() {
  if command -v curl >/dev/null 2>&1; then curl -fsSL "$1" -o "$2"
  elif command -v wget >/dev/null 2>&1; then wget -qO "$2" "$1"
  else fail "missing curl or wget"; fi
}

verify_crc() {
  set -- $(cksum "$1")
  actual_crc="${1:-}"; actual_size="${2:-}"
  set -- $(sed -n '1p' "$2")
  expected_crc="${1:-}"; expected_size="${2:-}"
  [ -n "$expected_crc" ] && [ -n "$expected_size" ] || fail "CRC sidecar invalid"
  [ "$actual_crc" = "$expected_crc" ] && [ "$actual_size" = "$expected_size" ] || fail "CRC check failed"
}

download_verified() {
  dest="$1"; sidecar="$2"; shift 2
  for url in "$@"; do
    log "Trying $url"
    if download_to "$url" "$dest" && download_to "$url.crc32" "$sidecar"; then
      verify_crc "$dest" "$sidecar"
      log "CRC check passed for ${dest##*/}"
      return 0
    fi
  done
  return 1
}

need uname; need mktemp; need tar; need chmod; need mkdir; need cp; need find; need head; need cksum; need sed

os="$(detect_os)"; arch="$(detect_arch)"
archive="share2us-gui_${os}_${arch}.tar.gz"

if [ "$version" = "latest" ]; then
  hosted_url="${base_url%/}/gui/downloads/$archive"
  github_url="https://github.com/$repo/releases/latest/download/$archive"
else
  hosted_url="${base_url%/}/gui/downloads/$version/$archive"
  github_url="https://github.com/$repo/releases/download/$version/$archive"
fi

tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT INT TERM

log "Downloading Share2Us GUI for $os/$arch..."
download_verified "$tmpdir/$archive" "$tmpdir/$archive.crc32" "$github_url" "$hosted_url" \
  || fail "could not download and verify $archive"

tar -xzf "$tmpdir/$archive" -C "$tmpdir"
bin="$tmpdir/$binary_name"
[ -f "$bin" ] || bin="$(find "$tmpdir" -type f -name "$binary_name" | head -n 1 || true)"
[ -n "$bin" ] || fail "archive did not contain $binary_name"

mkdir -p "$install_dir"
chmod 0755 "$bin"
cp "$bin" "$install_dir/$binary_name"

# Register the file-manager right-click integration (KDE ServiceMenu / Nemo /
# "Open With"). Best-effort — the app still works without it.
"$install_dir/$binary_name" --install-shell 2>/dev/null || true

log ""
log "Installed Share2Us GUI to $install_dir/$binary_name"
log "Right-click a file or folder in your file manager -> s2u -> Share."
log "Sign in from the app the first time (or run 's2u login' if you also have the CLI)."
