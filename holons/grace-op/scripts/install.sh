#!/usr/bin/env bash
# Install op — the Organic Programming CLI
#
# Usage:
#   /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/organic-programming/grace-op/dev/scripts/install.sh)"
#
# Flow:
#   1. Download op binary to a temp directory
#   2. Use temp op to run: op env --init (creates OPPATH, OPBIN, cache)
#   3. Copy binary from temp to OPBIN
#   4. Append shell snippet to profile (if not present)
#   5. Clean up temp
#
# Respects OPPATH and OPBIN if already set.

set -euo pipefail

REPO="organic-programming/grace-op"

# ── Detect platform ──────────────────────────────────────────

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$OS" in
  darwin) OS="darwin" ;;
  linux)  OS="linux" ;;
  *)      echo "✗ Unsupported OS: $OS"; exit 1 ;;
esac

case "$ARCH" in
  x86_64|amd64)  ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *)             echo "✗ Unsupported architecture: $ARCH"; exit 1 ;;
esac

# ── Download to temp ─────────────────────────────────────────

TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT

TMP_OP="$TMPDIR/op"
RELEASE_URL="https://github.com/$REPO/releases/latest/download/op_${OS}_${ARCH}"

echo "→ Installing op for ${OS}/${ARCH}..."

if curl -fsSL -o "$TMP_OP" "$RELEASE_URL" 2>/dev/null; then
  chmod +x "$TMP_OP"
  echo "✓ Downloaded op"
elif command -v go &>/dev/null; then
  echo "  No pre-built binary, building via go install..."
  GOBIN="$TMPDIR" go install "github.com/$REPO/cmd/op@latest"
  echo "✓ Built op"
else
  echo "✗ No pre-built binary and Go is not installed."
  echo "  Install Go from https://go.dev/dl/ or download op from:"
  echo "  https://github.com/$REPO/releases"
  exit 1
fi

# ── Let op set up its own environment ────────────────────────

"$TMP_OP" env --init

# Read OPBIN from op.
OPBIN="$("$TMP_OP" env 2>/dev/null | grep '^OPBIN=' | cut -d= -f2)"

echo "  OPBIN = $OPBIN"

# ── Install to OPBIN ────────────────────────────────────────

cp "$TMP_OP" "$OPBIN/op"
chmod +x "$OPBIN/op"
echo "✓ Installed op to $OPBIN/op"

# ── Shell integration ────────────────────────────────────────

SNIPPET='eval "$(op env --shell)"'

SHELL_NAME="$(basename "${SHELL:-/bin/bash}")"
case "$SHELL_NAME" in
  zsh)  PROFILE="$HOME/.zshrc" ;;
  bash) PROFILE="$HOME/.bashrc" ;;
  fish) PROFILE="$HOME/.config/fish/config.fish" ;;
  *)    PROFILE="$HOME/.profile" ;;
esac

if [ -f "$PROFILE" ] && grep -qF "op env --shell" "$PROFILE" 2>/dev/null; then
  echo "✓ Shell integration already in $PROFILE"
else
  {
    echo ""
    echo "# Organic Programming"
    echo "$SNIPPET"
  } >> "$PROFILE"
  echo "✓ Added shell integration to $PROFILE"
fi

# ── Done ─────────────────────────────────────────────────────

echo ""
echo "✓ Done. Restart your terminal, then run:"
echo "  op version"
echo ""
echo "Or activate now:"
echo "  eval \"\$(op env --shell)\""
