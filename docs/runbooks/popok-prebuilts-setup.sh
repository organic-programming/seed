#!/usr/bin/env bash
#
# popok-prebuilts-setup.sh
#
# Idempotent setup script for popok (Apple Silicon Mac, GitHub Actions
# self-hosted runner) to host the SDK prebuilts CI matrix.
#
# Companion to: docs/runbooks/popok-prebuilts-setup.md
#
# Usage:
#   bash popok-prebuilts-setup.sh                      # as popok user (preferred)
#   sudo bash popok-prebuilts-setup.sh                 # script will drop to popok user as needed
#
# What this script does (automated):
#   - Verifies preconditions (macOS arm64, popok user exists, free disk space)
#   - Installs Homebrew packages: colima, docker CLI, docker-buildx, docker-compose,
#     tart, syft, cosign, ninja, protobuf-c
#   - Installs Zig 0.16.0 to /Users/popok/tools
#   - Starts Colima with sane defaults for CI
#   - Registers QEMU binfmt handlers for cross-arch container builds
#   - Creates a launchd job for periodic Docker/Tart image cleanup
#
# What this script does NOT do (manual):
#   - Pull the Tart Windows VM image (1-2 hours, interactive progress)
#   - Configure Windows VM with Visual Studio Build Tools (interactive)
#   - Re-register the GitHub Actions runner with new labels (needs token from GitHub UI)
#   - Time the M0 Path A vs Path B Windows benchmark
#
# After this script completes successfully, follow sections 3 (Tart VM build), 5
# (runner re-registration), and 8 (smoke validation) of the markdown runbook
# manually.

set -euo pipefail

# ──────────────────────────────────────────────────────────────────────────────
# 0. Logging helpers
# ──────────────────────────────────────────────────────────────────────────────

log() { printf '\033[1;34m[setup]\033[0m %s\n' "$*"; }
ok()  { printf '\033[1;32m[ ok ]\033[0m %s\n' "$*"; }
warn(){ printf '\033[1;33m[warn]\033[0m %s\n' "$*" >&2; }
fail(){ printf '\033[1;31m[fail]\033[0m %s\n' "$*" >&2; exit 1; }

POPOK_USER="popok"
POPOK_HOME="/Users/${POPOK_USER}"
TOOLS_DIR="${POPOK_HOME}/tools"
LOCAL_BIN="${POPOK_HOME}/.local/bin"
ZIG_VERSION="0.16.0"

# ──────────────────────────────────────────────────────────────────────────────
# 1. Privilege handling
# ──────────────────────────────────────────────────────────────────────────────

# Run a command as the popok user. Used for all Homebrew + user-level operations.
run_as_popok() {
  if [[ "$(id -un)" == "${POPOK_USER}" ]]; then
    "$@"
  else
    sudo -u "${POPOK_USER}" -H bash -lc "$(printf '%q ' "$@")"
  fi
}

# ──────────────────────────────────────────────────────────────────────────────
# 2. Preflight checks
# ──────────────────────────────────────────────────────────────────────────────

preflight() {
  log "Preflight checks..."

  # macOS Apple Silicon
  [[ "$(uname -s)" == "Darwin" ]] || fail "Not macOS (got $(uname -s))"
  [[ "$(uname -m)" == "arm64" ]] || fail "Not Apple Silicon (got $(uname -m))"
  ok "macOS arm64 confirmed"

  # popok user exists
  if ! id -u "${POPOK_USER}" >/dev/null 2>&1; then
    fail "User '${POPOK_USER}' does not exist on this machine"
  fi
  ok "User '${POPOK_USER}' exists"

  # Disk space — at least 100 GB free recommended
  local free_gb
  free_gb=$(df -g / | awk 'NR==2 {print $4}')
  if (( free_gb < 100 )); then
    warn "Free disk space is ${free_gb} GB; at least 100 GB recommended (Tart image is ~30 GB)"
    read -r -p "Continue anyway? [y/N] " ans
    [[ "${ans:-N}" =~ ^[yY]$ ]] || fail "Aborted by user"
  else
    ok "Free disk space: ${free_gb} GB"
  fi

  # Homebrew installed
  if ! run_as_popok command -v brew >/dev/null 2>&1; then
    fail "Homebrew not installed for user '${POPOK_USER}'. Install from https://brew.sh first."
  fi
  ok "Homebrew available for ${POPOK_USER}"

  # GitHub Actions runner config exists (warn only, don't fail)
  if [[ ! -d "${POPOK_HOME}/actions-runner" ]]; then
    warn "No ~/actions-runner directory found for ${POPOK_USER} (expected for re-registration step)"
  fi
}

# ──────────────────────────────────────────────────────────────────────────────
# 3. Homebrew packages
# ──────────────────────────────────────────────────────────────────────────────

install_brew_packages() {
  log "Installing Homebrew packages..."

  local packages=(
    colima
    docker
    docker-buildx
    docker-compose
    cirruslabs/cli/tart
    syft
    cosign
    ninja
    protobuf-c
    grpc
    protobuf
  )

  # Update brew once at the top
  run_as_popok brew update >/dev/null

  for pkg in "${packages[@]}"; do
    if run_as_popok brew list --formula --cask "${pkg}" >/dev/null 2>&1; then
      ok "Already installed: ${pkg}"
    else
      log "Installing: ${pkg}"
      run_as_popok brew install "${pkg}"
    fi
  done

  # Buildx symlinks (Homebrew installs to /opt/homebrew/opt/docker-buildx; CLI looks in ~/.docker/cli-plugins)
  run_as_popok mkdir -p "${POPOK_HOME}/.docker/cli-plugins"
  run_as_popok ln -sfn /opt/homebrew/opt/docker-buildx/bin/docker-buildx \
    "${POPOK_HOME}/.docker/cli-plugins/docker-buildx"
  run_as_popok ln -sfn /opt/homebrew/opt/docker-compose/bin/docker-compose \
    "${POPOK_HOME}/.docker/cli-plugins/docker-compose"
  ok "Docker CLI plugins symlinked"
}

# ──────────────────────────────────────────────────────────────────────────────
# 4. Zig toolchain
# ──────────────────────────────────────────────────────────────────────────────

install_zig() {
  log "Installing Zig ${ZIG_VERSION}..."

  local zig_dir="${TOOLS_DIR}/zig-aarch64-macos-${ZIG_VERSION}"
  local zig_bin="${zig_dir}/zig"

  if [[ -x "${zig_bin}" ]]; then
    ok "Zig ${ZIG_VERSION} already installed at ${zig_bin}"
  else
    run_as_popok mkdir -p "${TOOLS_DIR}"
    local tarball="zig-aarch64-macos-${ZIG_VERSION}.tar.xz"
    run_as_popok bash -c "cd '${TOOLS_DIR}' && curl -fsSL -o '${tarball}' 'https://ziglang.org/download/${ZIG_VERSION}/${tarball}'"
    run_as_popok bash -c "cd '${TOOLS_DIR}' && tar -xf '${tarball}' && rm -f '${tarball}'"
    ok "Zig ${ZIG_VERSION} extracted to ${zig_dir}"
  fi

  run_as_popok mkdir -p "${LOCAL_BIN}"
  run_as_popok ln -sfn "${zig_bin}" "${LOCAL_BIN}/zig"
  ok "Symlinked ${LOCAL_BIN}/zig"

  # Verify
  local reported
  reported=$(run_as_popok "${zig_bin}" version)
  [[ "${reported}" == "${ZIG_VERSION}" ]] || fail "Zig version mismatch: got ${reported}, expected ${ZIG_VERSION}"
  ok "Zig version: ${reported}"
}

# ──────────────────────────────────────────────────────────────────────────────
# 5. Colima — container runtime
# ──────────────────────────────────────────────────────────────────────────────

start_colima() {
  log "Starting Colima..."

  # Already running?
  if run_as_popok colima status >/dev/null 2>&1; then
    ok "Colima already running"
  else
    run_as_popok colima start \
      --cpu 6 \
      --memory 12 \
      --disk 80 \
      --vm-type vz \
      --mount-type virtiofs \
      --arch aarch64 \
      --runtime docker
    ok "Colima started"
  fi

  # Persist across reboots
  if run_as_popok brew services list 2>/dev/null | grep -q '^colima.*started'; then
    ok "Colima already configured to start on boot"
  else
    run_as_popok brew services start colima || warn "Failed to register colima service"
  fi
}

# ──────────────────────────────────────────────────────────────────────────────
# 6. Buildx + multi-arch QEMU
# ──────────────────────────────────────────────────────────────────────────────

setup_buildx() {
  log "Configuring docker buildx..."

  if ! run_as_popok docker buildx ls 2>/dev/null | grep -q 'op-prebuilts'; then
    run_as_popok docker buildx create --name op-prebuilts --driver docker-container --use
    ok "Created buildx builder 'op-prebuilts'"
  else
    ok "Buildx builder 'op-prebuilts' already exists"
    run_as_popok docker buildx use op-prebuilts
  fi

  # Register QEMU binfmt handlers (idempotent — re-running is harmless)
  run_as_popok docker run --privileged --rm tonistiigi/binfmt --install all >/dev/null
  ok "QEMU binfmt handlers registered"

  run_as_popok docker buildx inspect --bootstrap >/dev/null
  ok "Buildx builder bootstrapped"
}

# ──────────────────────────────────────────────────────────────────────────────
# 7. Validation — Linux arm64 + amd64 round-trip
# ──────────────────────────────────────────────────────────────────────────────

validate_buildx() {
  log "Validating buildx cross-arch..."

  # Direct `docker run --platform <plat> alpine uname -m` exercises the QEMU
  # binfmt handlers without needing a Dockerfile or a build step.
  local amd64_arch arm64_arch

  amd64_arch=$(run_as_popok docker run --rm --platform linux/amd64 alpine:3.19 uname -m 2>/dev/null | tr -d '[:space:]')
  [[ "${amd64_arch}" == "x86_64" ]] || fail "linux/amd64 run returned '${amd64_arch}', expected 'x86_64'"
  ok "linux/amd64 run OK (uname -m → x86_64)"

  arm64_arch=$(run_as_popok docker run --rm --platform linux/arm64 alpine:3.19 uname -m 2>/dev/null | tr -d '[:space:]')
  [[ "${arm64_arch}" == "aarch64" ]] || fail "linux/arm64 run returned '${arm64_arch}', expected 'aarch64'"
  ok "linux/arm64 run OK (uname -m → aarch64)"
}

# ──────────────────────────────────────────────────────────────────────────────
# 8. Periodic cleanup launchd job (user-level, no sudo)
# ──────────────────────────────────────────────────────────────────────────────

install_cleanup_job() {
  log "Installing periodic cleanup launchd job..."

  local plist_dir="${POPOK_HOME}/Library/LaunchAgents"
  local plist="${plist_dir}/com.organic-programming.popok-prebuilts-cleanup.plist"
  local script_path="${TOOLS_DIR}/popok-prebuilts-cleanup.sh"

  run_as_popok mkdir -p "${plist_dir}" "${TOOLS_DIR}"

  # Write the cleanup script
  run_as_popok bash -c "cat > '${script_path}' <<'EOF'
#!/usr/bin/env bash
# popok-prebuilts-cleanup.sh — run weekly from launchd
set -euo pipefail
docker image prune -af --filter 'until=168h' >/dev/null 2>&1 || true
docker buildx prune -af --filter 'unused-for=168h' >/dev/null 2>&1 || true
# Tart: drop VMs not touched in 4 weeks
tart list 2>/dev/null | awk 'NR>1 && \$4 ~ /weeks|months/{print \$1}' | xargs -n1 -r tart delete 2>/dev/null || true
brew cleanup -s >/dev/null 2>&1 || true
EOF"
  run_as_popok chmod +x "${script_path}"

  # Write the launchd plist
  run_as_popok bash -c "cat > '${plist}' <<EOF
<?xml version='1.0' encoding='UTF-8'?>
<!DOCTYPE plist PUBLIC '-//Apple//DTD PLIST 1.0//EN' 'http://www.apple.com/DTDs/PropertyList-1.0.dtd'>
<plist version='1.0'>
<dict>
  <key>Label</key>
  <string>com.organic-programming.popok-prebuilts-cleanup</string>
  <key>ProgramArguments</key>
  <array>
    <string>${script_path}</string>
  </array>
  <key>StartCalendarInterval</key>
  <dict>
    <key>Weekday</key><integer>0</integer>
    <key>Hour</key><integer>3</integer>
    <key>Minute</key><integer>0</integer>
  </dict>
  <key>StandardOutPath</key>
  <string>${POPOK_HOME}/Library/Logs/popok-prebuilts-cleanup.log</string>
  <key>StandardErrorPath</key>
  <string>${POPOK_HOME}/Library/Logs/popok-prebuilts-cleanup.err.log</string>
</dict>
</plist>
EOF"

  # Load it (idempotent — unload first if already loaded)
  run_as_popok launchctl unload "${plist}" 2>/dev/null || true
  run_as_popok launchctl load "${plist}"
  ok "Cleanup job installed at ${plist} (runs Sunday 03:00)"
}

# ──────────────────────────────────────────────────────────────────────────────
# 9. Summary + manual next steps
# ──────────────────────────────────────────────────────────────────────────────

print_summary() {
  cat <<EOF

==============================================================================
popok prebuilts setup complete (automated portion)
==============================================================================

Installed and configured:
  ✓ Homebrew packages (colima, docker, buildx, tart, syft, cosign, ninja, ...)
  ✓ Zig ${ZIG_VERSION} at ${TOOLS_DIR}/zig-aarch64-macos-${ZIG_VERSION}
  ✓ Symlink ${LOCAL_BIN}/zig
  ✓ Colima running with 6 CPU / 12 GB RAM / 80 GB disk
  ✓ docker buildx 'op-prebuilts' with QEMU multi-arch
  ✓ Validated linux/amd64 + linux/arm64 round-trip
  ✓ Launchd cleanup job (weekly, Sunday 03:00)

Manual steps still required (see docs/runbooks/popok-prebuilts-setup.md):

  § 3  Tart Windows VM image pull (~1-2 hours, interactive)
       \$ tart clone ghcr.io/cirruslabs/windows:server-2022-with-buildtools \\
              windows-arm64-builder
       \$ tart set windows-arm64-builder --cpu 4 --memory 8192 --disk 60

  § 3bis Time the gRPC build under Tart vs native macOS to choose
         Path A (popok+Tart for Windows) vs Path B (GitHub windows-latest fallback)

  § 5  Re-register GitHub Actions runner with the new labels:
       \$ cd ~/actions-runner
       \$ ./config.sh remove --token <REMOVAL_TOKEN_FROM_GITHUB_UI>
       \$ ./config.sh \\
            --url https://github.com/organic-programming/seed \\
            --token <REGISTRATION_TOKEN_FROM_GITHUB_UI> \\
            --name popok \\
            --labels self-hosted,popok,macos,linux-via-docker,windows-vm \\
            --work _work --runasservice

  § 8  End-to-end smoke validation script (manual run after § 3 and § 5).

Logs: ${POPOK_HOME}/Library/Logs/popok-prebuilts-cleanup.log

==============================================================================
EOF
}

# ──────────────────────────────────────────────────────────────────────────────
# Main
# ──────────────────────────────────────────────────────────────────────────────

main() {
  log "Starting popok prebuilts setup..."
  log "Running as: $(id -un) (UID $(id -u))"
  if [[ "$(id -un)" != "${POPOK_USER}" && "${EUID}" -ne 0 ]]; then
    fail "Must run as ${POPOK_USER} or via sudo. Got: $(id -un)"
  fi

  preflight
  install_brew_packages
  install_zig
  start_colima
  setup_buildx
  validate_buildx
  install_cleanup_job
  print_summary

  ok "Done."
}

main "$@"
