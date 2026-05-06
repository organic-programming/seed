#!/usr/bin/env bash
set -euo pipefail

# Setup POPOK NOCTAMBULE for codegen distribution builds.
#
# Idempotent: safe to re-run on the macOS runner.
#
# Manual prerequisites that this script cannot safely automate:
# - Xcode or Command Line Tools installation and `sudo xcodebuild -license accept`.
# - Apple Developer account, signing certificates, provisioning profiles, and Keychain access.
# - Network/proxy credentials for GitHub, Maven Central, npm, pub.dev, RubyGems, NuGet, and SwiftPM.
# - Repository checkout plus initialized submodules (`git submodule update --init --recursive`).

log() {
  printf '==> %s\n' "$*"
}

die() {
  printf 'error: %s\n' "$*" >&2
  exit 1
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "missing required command: $1"
}

install_formula() {
  local formula="$1"
  if brew list --versions "$formula" >/dev/null 2>&1; then
    log "brew formula already installed: $formula"
    return
  fi
  log "brew install $formula"
  brew install --quiet "$formula"
}

install_cask() {
  local cask="$1"
  if brew list --cask --versions "$cask" >/dev/null 2>&1; then
    log "brew cask already installed: $cask"
    return
  fi
  log "brew install --cask $cask"
  brew install --cask "$cask"
}

append_ci_env() {
  local key="$1"
  local value="$2"
  if [[ -n "${GITHUB_ENV:-}" ]]; then
    printf '%s=%s\n' "$key" "$value" >>"$GITHUB_ENV"
  fi
}

prepend_path() {
  local dir="$1"
  if [[ -d "$dir" ]]; then
    export PATH="$dir:$PATH"
    if [[ -n "${GITHUB_PATH:-}" ]]; then
      printf '%s\n' "$dir" >>"$GITHUB_PATH"
    fi
  fi
}

[[ "$(uname -s)" == "Darwin" ]] || die "POPOK setup expects macOS"
require_cmd brew

if ! xcode-select -p >/dev/null 2>&1; then
  die "Xcode Command Line Tools are not selected; run: xcode-select --install"
fi
require_cmd xcodebuild
require_cmd swift

log "installing Homebrew packages"
brew tap dart-lang/dart >/dev/null 2>&1 || true
install_formula cmake
install_formula ninja
install_formula go
install_formula node
install_formula zig
install_formula dart-lang/dart/dart
install_formula dotnet
install_formula gradle
install_formula openjdk@21
install_formula ruby@3.1
install_formula xcodegen
install_formula rustup-init

if ! command -v flutter >/dev/null 2>&1; then
  install_cask flutter
fi

java_home="$(brew --prefix openjdk@21)/libexec/openjdk.jdk/Contents/Home"
export JAVA_HOME="$java_home"
append_ci_env JAVA_HOME "$java_home"
prepend_path "$java_home/bin"

ruby_prefix="$(brew --prefix ruby@3.1)"
prepend_path "$ruby_prefix/bin"

if ! command -v cargo >/dev/null 2>&1; then
  log "initializing rustup stable toolchain"
  rustup-init -y --no-modify-path --default-toolchain stable
  prepend_path "$HOME/.cargo/bin"
else
  rustup toolchain install stable --profile minimal
fi

log "warming package-manager caches used by codegen builds"
npm --version >/dev/null
dart pub cache repair >/dev/null 2>&1 || true
swift package --version >/dev/null

log "verification"
xcodebuild -version
swift --version
go version
cmake --version | head -1
printf 'ninja %s\n' "$(ninja --version)"
java --version
gradle --version | sed -n '1,3p'
node --version
npm --version
zig version
dart --version
flutter --version | sed -n '1,2p'
dotnet --version
ruby -v
bundle -v
cargo --version
rustc --version
xcodegen --version

log "POPOK NOCTAMBULE setup complete"
