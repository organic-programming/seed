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

  # `docker buildx inspect` returns 0 iff the builder exists. More reliable
  # than parsing `docker buildx ls` (which formats differently under TTY vs
  # non-TTY).
  if run_as_popok docker buildx inspect op-prebuilts >/dev/null 2>&1; then
    ok "Buildx builder 'op-prebuilts' already exists"
    run_as_popok docker buildx use op-prebuilts
  else
    run_as_popok docker buildx create --name op-prebuilts --driver docker-container --use
    ok "Created buildx builder 'op-prebuilts'"
  fi

  # Register QEMU binfmt handlers (idempotent — re-running is harmless)
  run_as_popok docker run --privileged --rm tonistiigi/binfmt --install all >/dev/null
  ok "QEMU binfmt handlers registered"

  run_as_popok docker buildx inspect --bootstrap op-prebuilts >/dev/null
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

pull_tart_vm() {
  log "Attempting Tart Windows VM image pull (~30 GB if successful)..."

  if run_as_popok tart list 2>/dev/null | awk '{print $2}' | grep -qx 'windows-arm64-builder'; then
    ok "Tart VM 'windows-arm64-builder' already present"
    run_as_popok tart set windows-arm64-builder --cpu 4 --memory 8192 --disk 60
    ok "Tart VM configured (4 CPU, 8 GB RAM, 60 GB disk)"
    return
  fi

  # The pre-built `windows:server-2022-with-buildtools` image is Cirrus's
  # commercial offering and requires a Cirrus Runners subscription. The pull
  # returns 403 without auth. Treat as non-fatal: the macOS-native bench still
  # runs, and the Windows decision can default to Path B (GitHub windows-latest
  # fallback) until/unless a Cirrus subscription or custom Windows VM image is
  # available.
  if run_as_popok tart clone \
       ghcr.io/cirruslabs/windows:server-2022-with-buildtools \
       windows-arm64-builder 2>/dev/null; then
    run_as_popok tart set windows-arm64-builder --cpu 4 --memory 8192 --disk 60
    ok "Tart VM 'windows-arm64-builder' cloned and configured"
  else
    warn "Tart pull failed (likely 403: image requires Cirrus subscription)."
    warn "Skipping Windows VM setup. Path B (GitHub windows-latest) will be the default for the Windows target in the prebuilts chantier."
    warn "If you obtain a Cirrus subscription later, run: tart login ghcr.io <user> <pat> && bash $(realpath "$0")"
    warn "Alternative: build a custom Windows 11 ARM64 VM locally with Visual Studio Build Tools (interactive, ~2h manual work)."
  fi
}

run_grpc_bench_macos() {
  log "Running gRPC build benchmark on native macOS (baseline for Path A vs Path B Windows decision)..."

  local bench_dir="${TOOLS_DIR}/grpc-bench"
  local grpc_src="${bench_dir}/grpc-src"
  local result_file="${bench_dir}/macos-native-build-time.txt"

  run_as_popok mkdir -p "${bench_dir}"

  if [[ -d "${grpc_src}/.git" ]]; then
    ok "gRPC source already cloned at ${grpc_src}"
  else
    run_as_popok git clone --depth 1 --branch v1.80.0 \
      https://github.com/grpc/grpc "${grpc_src}"
    log "Initialising gRPC submodules (~500 MB, 10-15 min)..."
    run_as_popok git -C "${grpc_src}" submodule update --init --recursive --jobs 4
    ok "gRPC source + submodules ready"
  fi

  if [[ -f "${result_file}" ]]; then
    ok "macOS-native gRPC build already benchmarked: $(cat "${result_file}")"
    return
  fi

  log "Building gRPC native macOS arm64 (this takes 10-20 min, please be patient)..."
  local build_dir="${grpc_src}/build-bench"
  run_as_popok rm -rf "${build_dir}"
  run_as_popok mkdir -p "${build_dir}"

  # Capture wall-clock time. Output stored to result_file for later comparison.
  local start_ts end_ts elapsed
  start_ts=$(date +%s)
  run_as_popok bash -c "cd '${build_dir}' && cmake -G Ninja -DCMAKE_BUILD_TYPE=Release -DgRPC_BUILD_TESTS=OFF -DgRPC_BUILD_CODEGEN=OFF '${grpc_src}' >/dev/null && cmake --build . --target grpc -j 4 >/dev/null"
  end_ts=$(date +%s)
  elapsed=$((end_ts - start_ts))

  run_as_popok bash -c "echo 'macos-arm64-native: ${elapsed}s ('$((elapsed / 60))'m '$((elapsed % 60))'s)' > '${result_file}'"
  ok "macOS-native gRPC build done in ${elapsed}s ($((elapsed / 60))m $((elapsed % 60))s) — saved to ${result_file}"
}

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

Two manual steps remain (cannot be scripted — need GitHub UI tokens or human
decision):

  1. Re-register the GitHub Actions runner with the new labels.
     The existing runner is at /Users/popok/code/actions-runner.
     Get a registration token from GitHub: Settings → Actions → Runners →
     New self-hosted runner. Then:

       \$ cd /Users/popok/code/actions-runner
       \$ sudo ./svc.sh stop
       \$ ./config.sh \\
            --url https://github.com/organic-programming/seed \\
            --token <TOKEN_FROM_GITHUB_UI> \\
            --name popok \\
            --labels self-hosted,popok,macos,linux-via-docker,windows-vm \\
            --work _work \\
            --replace
       \$ sudo ./svc.sh start

  2. Decide Path A vs Path B for Windows builds.
     The macOS-native gRPC build time is in:
       ${TOOLS_DIR}/grpc-bench/macos-native-build-time.txt
     To get the Windows-side number, boot the VM and run the same build:
       \$ tart run windows-arm64-builder --no-graphics &
       \$ ssh Administrator@\$(tart ip windows-arm64-builder)
       (inside the VM, run a Windows-equivalent gRPC build via vcpkg or cmake)
     If Windows time ≤ 2× macOS time, choose Path A (popok+Tart for Windows).
     Otherwise choose Path B (GitHub windows-latest fallback for Windows only).
     Record decision in docs/adr/sdk-prebuilts-scope.md when Codex's M0 ADR lands.

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
  pull_tart_vm
  run_grpc_bench_macos
  install_cleanup_job
  print_summary

  ok "Done."
}

main "$@"
