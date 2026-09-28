#!/bin/sh
set -eu

# jobs-cli installer
# Usage: curl -fsSL https://raw.githubusercontent.com/thedavidweng/jobs-cli/main/install.sh | sh

REPO="thedavidweng/jobs-cli"
BINARY="jobs-cli"
CASK="thedavidweng/tap/jobs-cli"
BLOCK_START="# >>> jobs-cli >>>"
BLOCK_END="# <<< jobs-cli <<<"

step()  { printf '==> %s\n' "$1"; }
die()   { printf 'ERROR: %s\n' "$1" >&2; exit 1; }

# --- Detect OS and ARCH ---
os="$(uname -s)"
arch="$(uname -m)"

case "$os" in
  Darwin) platform="darwin" ;;
  Linux)  platform="linux"  ;;
  *)      die "Unsupported OS: $os. Use install.ps1 on Windows." ;;
esac

case "$arch" in
  x86_64|amd64)  goarch="x86_64" ;;
  arm64|aarch64) goarch="arm64" ;;
  *)             die "Unsupported architecture: $arch" ;;
esac

platform_label="$platform/$goarch"

# --- Resolve latest version ---
resolve_version() {
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name"' | head -1 | sed 's/.*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/'
  elif command -v wget >/dev/null 2>&1; then
    wget -q -O - "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name"' | head -1 | sed 's/.*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/'
  else
    die "curl or wget is required."
  fi
}

download() {
  url="$1"
  output="$2"
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$url" -o "$output"
  elif command -v wget >/dev/null 2>&1; then
    wget -q -O "$output" "$url"
  else
    die "curl or wget is required."
  fi
}

# --- Check for Homebrew ---
has_brew() {
  command -v brew >/dev/null 2>&1
}

install_via_brew() {
  step "Installing via Homebrew Cask (thedavidweng/tap)"
  brew tap "thedavidweng/tap" 2>/dev/null || true
  brew install --cask "$CASK"
}

install_binary() {
  version="$1"

  case "$platform" in
    darwin) asset="${BINARY}_darwin_universal.tar.gz" ;;
    linux)  asset="${BINARY}_linux_${goarch}.tar.gz" ;;
    *)      die "Unsupported platform: $platform" ;;
  esac

  url="https://github.com/$REPO/releases/download/$version/$asset"

  bin_dir="${JOBS_INSTALL_DIR:-$HOME/.local/bin}"
  mkdir -p "$bin_dir"

  tmp_dir="$(mktemp -d)"
  trap 'rm -rf "$tmp_dir"' EXIT INT TERM

  step "Downloading $asset"
  download "$url" "$tmp_dir/$asset"

  step "Installing to $bin_dir/$BINARY"
  tar -xzf "$tmp_dir/$asset" -C "$tmp_dir"
  chmod +x "$tmp_dir/$BINARY"
  mv -f "$tmp_dir/$BINARY" "$bin_dir/$BINARY"

  # Add to PATH if needed
  case ":$PATH:" in
    *":$bin_dir:"*) ;;
    *)
      shell_profile=""
      case "$platform:${SHELL:-}" in
        darwin:*/zsh)  shell_profile="$HOME/.zprofile" ;;
        darwin:*/bash) shell_profile="$HOME/.bash_profile" ;;
        linux:*/zsh)   shell_profile="$HOME/.zshrc" ;;
        linux:*/bash)  shell_profile="$HOME/.bashrc" ;;
        *)             shell_profile="$HOME/.profile" ;;
      esac

      path_line="export PATH=\"$bin_dir:\$PATH\""
      if [ -f "$shell_profile" ] && grep -qxF -e "$path_line" "$shell_profile"; then
        step "$shell_profile already adds $bin_dir to PATH"
      elif printf '\n%s\n%s\n%s\n' "$BLOCK_START" "$path_line" "$BLOCK_END" 2>/dev/null >> "$shell_profile"; then
        step "Added $bin_dir to PATH in $shell_profile"
      else
        step "Could not update $shell_profile. Add $bin_dir to your PATH yourself."
      fi
      step "Run: export PATH=\"$bin_dir:\$PATH\" to use in current terminal"
      ;;
  esac

  step "Installed $("${bin_dir}/${BINARY}" --version 2>/dev/null || echo "$version")"
}

# --- Uninstall helper ---
uninstall_brew() {
  step "Uninstalling Homebrew-managed jobs-cli"
  brew uninstall --cask "$CASK" 2>/dev/null || true
  brew untap "thedavidweng/tap" 2>/dev/null || true
}

uninstall_binary() {
  bin_dir="${JOBS_INSTALL_DIR:-$HOME/.local/bin}"
  if [ -f "$bin_dir/$BINARY" ]; then
    step "Removing $bin_dir/$BINARY"
    rm -f "$bin_dir/$BINARY"
  fi
  remove_path_block "$bin_dir"
}

# Other programs in the directory may rely on the PATH entry, so the block
# goes only once the directory is empty.
remove_path_block() {
  path_line="export PATH=\"$1:\$PATH\""
  tmp_file="$(mktemp)"
  trap 'rm -f "$tmp_file"' EXIT INT TERM
  for profile in "$HOME/.zprofile" "$HOME/.bash_profile" "$HOME/.zshrc" "$HOME/.bashrc" "$HOME/.profile"; do
    [ -f "$profile" ] || continue
    # Drops the block for this directory and the blank line written before
    # it. Exits 1 when there is no such block.
    PATH_LINE="$path_line" awk -v start="$BLOCK_START" -v end="$BLOCK_END" '
      in_block {
        block = block "\n" $0
        if ($0 == ENVIRON["PATH_LINE"]) found = 1
        if ($0 == end) {
          in_block = 0
          if (found) removed = 1
          else { if (blank) print ""; print block }
        }
        next
      }
      $0 == start {
        blank = held && prev == ""
        if (held && !blank) print prev
        held = 0; in_block = 1; found = 0; block = $0
        next
      }
      { if (held) print prev; prev = $0; held = 1 }
      END {
        if (in_block) { if (blank) print ""; print block }
        else if (held) print prev
        exit removed ? 0 : 1
      }
    ' "$profile" > "$tmp_file" || continue
    if [ -n "$(ls -A "$1" 2>/dev/null)" ]; then
      step "Kept the PATH entry in $profile because $1 is not empty"
    elif cat "$tmp_file" 2>/dev/null > "$profile"; then
      step "Removed $1 from PATH in $profile"
    else
      step "Could not update $profile. Remove the $BLOCK_START block yourself."
    fi
  done
}

# --- Main ---
case "${1:-}" in
  uninstall)
    if has_brew && brew list --cask "$BINARY" >/dev/null 2>&1; then
      uninstall_brew
    elif has_brew && brew list --formula "$BINARY" >/dev/null 2>&1; then
      step "Uninstalling Homebrew formula for jobs-cli"
      brew uninstall --formula "$BINARY"
    else
      uninstall_binary
    fi
    case "$platform:${XDG_CONFIG_HOME:-}" in
      darwin:*) config_dir="$HOME/.jobs-cli" ;;
      linux:/*) config_dir="$XDG_CONFIG_HOME/jobs-cli" ;;
      *)        config_dir="$HOME/.config/jobs-cli" ;;
    esac
    config_dir="${JOBS_CONFIG_DIR:-$config_dir}"
    step "Uninstalled. You may also remove jobs-cli config and sessions from $config_dir/"
    exit 0
    ;;
  --help|-h)
    cat <<EOF
Usage: install.sh [uninstall]

Installs jobs-cli. Prefers Homebrew Cask if available, otherwise
downloads the binary to ~/.local/bin.

Environment:
  JOBS_INSTALL_DIR  Directory for binary (default: ~/.local/bin)

Options:
  uninstall    Remove jobs-cli
  --help, -h   Show this help
EOF
    exit 0
    ;;
esac

step "Installing jobs-cli ($platform_label)"

if has_brew; then
  install_via_brew
else
  version="$(resolve_version)"
  [ -z "$version" ] && die "Could not resolve latest version."
  step "Latest version: $version"
  install_binary "$version"
fi

printf '\n'
step "Run 'jobs-cli doctor' to check your setup."
step "Run 'jobs-cli --help' to see available commands."
