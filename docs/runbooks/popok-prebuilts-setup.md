# Popok preparation — SDK prebuilts chantier

Status: runbook  
Date: 2026-04-26  
References: [`docs/specs/sdk-prebuilts.md`](../specs/sdk-prebuilts.md), [`docs/audit/sdk-toolchain-audit.md`](../audit/sdk-toolchain-audit.md), [`.github/workflows/ader.yml`](../../.github/workflows/ader.yml)

This runbook prepares `popok` (the existing self-hosted Apple Silicon Mac runner) to host the SDK prebuilts CI matrix per the resolved spec §6.2 and §11.2: popok hosts every target via containers (Linux) and VMs (Windows), with GitHub-hosted runners reserved as the last-resort fallback.

Execute the sections in order. Each ends with a validation step you can run and verify before moving on.

---

## 0. Inventory — what popok already has

Per [`.github/workflows/ader.yml`](../../.github/workflows/ader.yml), popok is already configured with:

- macOS Apple Silicon, GitHub Actions self-hosted runner with label `popok`
- Cache root at `/Users/popok/.ader-ci-cache/` with subdirectories: `go-mod`, `go-build`, `bundle`, `dart-pub`, `npm`, `gradle`, `nuget`, `dotnet-home`, `grace-op-shared`
- Toolchains on PATH: `go`, `ruby`, `bundle`, `swift`, `dart`, `flutter`, `cargo`, `python3`, `cmake`, `dotnet`, `java`, `gradle`, `node`, `npm`, `xcodebuild`, `xcodegen`, `codesign`
- Homebrew with at least: `grpc` and `protobuf` (per `cpp-holons/CMakeLists.txt:13` finding `/opt/homebrew/lib`)

Confirm before starting:

```bash
# As popok user on popok itself
sw_vers                                              # macOS version, expect 14+
uname -m                                             # arm64
brew --version                                       # 4.x
ls /Users/popok/.ader-ci-cache/                      # caches present
gh actions list --self-hosted 2>/dev/null || \
  systemctl --user list-units 'actions.runner.*'     # runner registered
```

---

## 1. Disk-space budget

| Component | Estimated size | Notes |
|---|---|---|
| Docker runtime + image cache | 5 GB initial, grows to 20 GB | Mostly gRPC-from-source build outputs |
| Tart VM images (Windows base) | 30 GB per VM image | Windows 11 ARM64 + Visual Studio Build Tools |
| Per-build vendored gRPC + protobuf-c | 2 GB × N concurrent builds | Cleaned after each build |
| Prebuilts output cache | 5 GB | Tarballs awaiting upload to GitHub Releases |
| Existing `ader-ci-cache` | already in place | Don't shrink |
| **Total headroom required** | **~80 GB free** | After existing caches |

Verify:

```bash
df -h /                                              # popok free space
du -sh /Users/popok/.ader-ci-cache                   # current cache size
```

If free space is below 100 GB, plan storage cleanup or expansion before continuing.

---

## 2. Container runtime: Colima

Recommendation: **Colima** over Docker Desktop. Reasons:
- No license fee, no GUI, no menubar agent.
- Scriptable, headless, fits CI semantics.
- Supports Docker CLI compatibility, BuildKit, multi-arch via QEMU.
- Apple Silicon native; emulates `linux/amd64` cleanly.

### Install

```bash
brew install colima docker docker-buildx docker-compose
mkdir -p ~/.docker/cli-plugins
ln -sfn /opt/homebrew/opt/docker-buildx/bin/docker-buildx ~/.docker/cli-plugins/docker-buildx
ln -sfn /opt/homebrew/opt/docker-compose/bin/docker-compose ~/.docker/cli-plugins/docker-compose
```

### Configure for CI use

Allocate enough resources to host 2-3 concurrent Linux builds:

```bash
colima start \
  --cpu 6 \
  --memory 12 \
  --disk 80 \
  --vm-type vz \
  --mount-type virtiofs \
  --arch aarch64 \
  --runtime docker
```

`--vm-type vz` uses Apple's Virtualization.framework (faster than QEMU). `--arch aarch64` makes the VM ARM-native; `linux/amd64` jobs run via Rosetta-style emulation.

To make it persist across reboots:

```bash
brew services start colima
```

### Enable cross-arch via buildx

```bash
docker buildx create --name op-prebuilts --driver docker-container --use
docker run --privileged --rm tonistiigi/binfmt --install all   # registers QEMU handlers
docker buildx inspect --bootstrap
```

### Validation

Test that both architectures build successfully:

```bash
mkdir -p /tmp/colima-validate && cd /tmp/colima-validate
cat > Dockerfile <<'EOF'
FROM alpine:3.19
RUN uname -m && cat /etc/alpine-release
EOF

docker buildx build --platform linux/amd64 --load -t test:amd64 .
docker run --rm test:amd64                            # expects: x86_64
docker buildx build --platform linux/arm64 --load -t test:arm64 .
docker run --rm test:arm64                            # expects: aarch64
```

If both `uname -m` outputs are correct, Colima is ready.

---

## 3. Virtualization runtime: Tart (for Windows target)

Recommendation: **Tart** over UTM. Reasons:
- Purpose-built for CI, CLI-only, no GUI.
- Same publisher (Cirrus Labs) as Tart-managed self-hosted runners — battle-tested for Apple Silicon.
- Pulls VM images from OCI registries (similar to Docker), versionable.
- Free for open-source use; commercial license needed for closed-source production CI (does not apply to seed since it's public).

### Install

```bash
brew install cirruslabs/cli/tart
tart --version
```

### Pull a Windows 11 ARM64 base image with Visual Studio Build Tools

The seed chantier needs MSVC compatibility for `x86_64-pc-windows-msvc`. Windows 11 ARM64 runs MSVC natively for ARM64 binaries; the x86_64 Windows target requires either x86 emulation (slower) or building x86_64 from a Windows ARM64 host with MSVC's cross-compile capability (preferred).

```bash
# Pull a pre-built Windows 11 ARM64 image with VS Build Tools
tart clone ghcr.io/cirruslabs/windows:server-2022-with-buildtools windows-arm64-builder

# OR build from base if a custom toolchain is needed
tart clone ghcr.io/cirruslabs/windows:11 windows-base
tart run windows-base --no-graphics &
# Inside: install Build Tools 2022 with C++ workload, then sysprep + shutdown
```

### Configure VM resources

```bash
tart set windows-arm64-builder --cpu 4 --memory 8192 --disk 60
```

### Validation

```bash
tart run windows-arm64-builder --no-graphics &
TART_VM_IP=$(tart ip windows-arm64-builder)
ssh-keyscan -H "$TART_VM_IP" >> ~/.ssh/known_hosts
ssh "Administrator@$TART_VM_IP" 'cl.exe' 2>&1 | head -3
# Expect: "Microsoft (R) C/C++ Optimizing Compiler Version 19.x for ARM64 / x64"
tart stop windows-arm64-builder
```

If `cl.exe` responds, the Windows toolchain is reachable.

### M0 timing test (Path A vs Path B decision)

Per spec §6.2 and the M0 phase of the Codex prompt, time a representative Windows build under Tart and compare to native macOS:

```bash
# Native macOS arm64 build of gRPC (just the smoke target, not full)
time ( cd /tmp && git clone --depth 1 --branch v1.80.0 https://github.com/grpc/grpc grpc-bench && \
       cd grpc-bench && cmake -B build -G Ninja -DgRPC_BUILD_TESTS=OFF -DCMAKE_BUILD_TYPE=Release && \
       cmake --build build --target grpc -j 4 )

# Same build inside the Tart Windows VM
tart run windows-arm64-builder --no-graphics &
ssh "Administrator@$TART_VM_IP" 'time (...same gRPC build script under cmd or powershell...)'
```

If Windows-VM time ≤ 2× native time, choose **Path A** (popok+Tart for Windows). Otherwise **Path B** (GitHub `windows-latest` for Windows only). Record the decision in `docs/adr/sdk-prebuilts-scope.md` during M0.

---

## 4. Build tools needed beyond the existing toolchain

### `syft` — SBOM generator

```bash
brew install syft
syft --version                                       # 1.x
```

### `cosign` — artifact signing (v1.0+ only, can install now)

```bash
brew install cosign
cosign version
```

V0.x of the prebuilts ships SHA-256 only (no signing). Install cosign now to be ready when v1.0 lands.

### `ninja` — required by Zig SDK build

Likely already present (CMake on Apple Silicon often installs ninja transitively), but confirm:

```bash
brew install ninja
ninja --version                                      # 1.11+
```

### `protoc-c` — libprotobuf-c codegen plugin (for the c/cpp/zig prebuilts validation)

```bash
brew install protobuf-c
which protoc-c                                       # /opt/homebrew/bin/protoc-c
```

Needed to validate that the prebuilt archives' `bin/protoc-c` work after extraction on a clean machine — popok itself doesn't need it for the runtime, but it's the smoke-test reference binary.

### Zig 0.16.0 — for the Zig SDK target

If not already on popok from the Zig chantier:

```bash
mkdir -p /Users/popok/tools
cd /Users/popok/tools
curl -O https://ziglang.org/download/0.16.0/zig-aarch64-macos-0.16.0.tar.xz
tar -xf zig-aarch64-macos-0.16.0.tar.xz
ln -sfn /Users/popok/tools/zig-aarch64-macos-0.16.0/zig /Users/popok/.local/bin/zig
zig version                                          # 0.16.0
```

Add `/Users/popok/.local/bin` to PATH for the runner if not already.

### Validation

```bash
for cmd in syft cosign ninja protoc-c zig; do
  which "$cmd" && "$cmd" --version 2>&1 | head -1 || echo "MISSING: $cmd"
done
```

All five must resolve.

---

## 5. GitHub Actions runner labels

Current registration: label `popok`.

The prebuilts workflows expect more granular labels per target type. Two acceptable approaches:

### Option A (preferred): one runner with multiple labels

Edit the runner's `.runner` config (typically at `~/actions-runner/.runner`) and add labels. **Or** re-register the runner with the additional labels:

```bash
cd ~/actions-runner
./config.sh remove --token <REMOVAL_TOKEN>           # get token from GitHub repo settings
./config.sh \
  --url https://github.com/organic-programming/seed \
  --token <REGISTRATION_TOKEN> \
  --name popok \
  --labels self-hosted,popok,macos,linux-via-docker,windows-vm \
  --work _work \
  --runasservice
```

Then `runs-on: [self-hosted, popok, macos]` matches; `runs-on: [self-hosted, popok, windows-vm]` also matches the same runner. Workflows pick by intersection of labels.

### Option B: multiple runners on the same machine

If parallel builds are wanted, register two or three runners with different label sets:

```bash
mkdir -p ~/actions-runner-2 && cd ~/actions-runner-2
# ...same config script with --name popok-2 ...
```

Each runner consumes some CPU/RAM independently. With 6 CPUs on Colima for Linux + 4 for Tart + headroom for macOS, popok can run 2-3 concurrent builds. Tune to taste after observing real workload.

### Validation

After re-registration, in the GitHub repo's Actions → Runners page, confirm that `popok` shows the new labels: `self-hosted`, `popok`, `macos`, `linux-via-docker`, `windows-vm`.

Trigger a no-op smoke workflow (workflow_dispatch on `.github/workflows/sdk-prebuilts.yml` once it lands) targeting each label. Each must pick up on popok.

---

## 6. Network and security

### Egress

popok needs outbound HTTPS to:
- `github.com` and `objects.githubusercontent.com` (artifact uploads, source pulls)
- `ghcr.io` (Tart image pulls)
- `*.docker.io` and `registry-1.docker.io` (Colima image pulls)
- `ziglang.org` (Zig toolchain)
- `homebrew.sh` redirects (Homebrew updates)

If popok is behind a corporate firewall, allowlist these. The Tart and Colima image pulls are big (multi-GB on first install).

### Container isolation

Per spec §10:

```bash
# Verify containers cannot access popok's host filesystem outside what's mounted
docker run --rm alpine:3.19 ls /Users/popok 2>&1     # expect: No such file
docker run --rm alpine:3.19 ls /var/folders/ 2>&1    # expect: No such file
```

The Colima `--mount-type virtiofs` only mounts what's explicitly declared. Default mounts include `/Users` (read-only), but builds should not depend on that — workflow steps should bind-mount explicit paths only.

### Tart VM isolation

Tart VMs share no filesystem with popok by default. If a workflow needs file exchange, use `tart push` / `tart pull` or scp over the VM's IP.

### Secrets

The Actions runner stores no static secrets in `~/actions-runner`. All secrets come from GitHub Actions per-run via `${{ secrets.X }}`. Verify there are no `.env` files or stored credentials on popok outside the runner's standard work directory:

```bash
find ~ -name ".env*" -type f 2>/dev/null
find ~ -name "credentials*" -type f 2>/dev/null
```

If any unexpected files appear, inspect and remove.

---

## 7. Operational hygiene

### Image rotation

Docker images, Tart images, and Zig toolchain releases will accumulate. Schedule periodic cleanup:

```bash
# Weekly cron (or run manually)
docker image prune -af --filter "until=168h"         # purge images > 7 days old
tart list | awk 'NR>1 && $4 ~ /weeks|months/{print $1}' | xargs -n1 tart delete
brew cleanup -s
```

### Build duration tracking

Add per-target build timing to the prebuilts workflows. Output captured to `dist/<sdk>-<target>-buildtime.txt` and uploaded as an artifact. After several runs, populate a durations table in the spec for capacity planning.

### Disk-space alarms

```bash
# Add to a launchd job or simple cron
df -h / | awk 'NR==2 && int($5) > 80 {print "WARN: popok disk > 80% used"; exit 1}'
```

### Runner health

```bash
# Check the runner is registered and online
launchctl list | grep -i actions.runner
# OR for systemd-style on macOS:
sudo launchctl print system/actions.runner.popok 2>&1 | head -10
```

---

## 8. Smoke validation — end-to-end

Once sections 1–6 are done, run this sequence to confirm popok is ready:

```bash
# 1. Clone seed
mkdir -p /tmp/popok-validate && cd /tmp/popok-validate
git clone https://github.com/organic-programming/seed.git
cd seed

# 2. macOS arm64 build (native)
brew install grpc protobuf-c
gcc --version

# 3. Linux amd64 build (Docker emulated)
docker run --rm --platform linux/amd64 \
  -v "$PWD:/seed" -w /seed \
  alpine:3.19 sh -c 'apk add --no-cache build-base cmake ninja git && \
                     git submodule update --init --recursive sdk/zig-holons/third_party/protobuf-c 2>&1 | tail -5'

# 4. Linux arm64 build (Docker native)
docker run --rm --platform linux/arm64 \
  -v "$PWD:/seed" -w /seed \
  alpine:3.19 sh -c 'apk add --no-cache build-base cmake ninja git && uname -m'

# 5. Windows VM boot + cl.exe presence
tart run windows-arm64-builder --no-graphics &
sleep 30
TART_IP=$(tart ip windows-arm64-builder)
ssh "Administrator@$TART_IP" 'cl.exe' 2>&1 | head -3
tart stop windows-arm64-builder

# 6. Tools all present
for cmd in syft cosign ninja protoc-c zig; do which "$cmd" || echo "MISSING: $cmd"; done

# 7. Runner labels reach intended workflows
gh api repos/organic-programming/seed/actions/runners --jq '.runners[] | select(.name=="popok") | .labels'
# Expect: ["self-hosted","popok","macos","linux-via-docker","windows-vm"]
```

If all 7 steps pass, popok is ready. If any fail, fix that step before continuing — do not proceed to the Codex chantier with a partial setup.

---

## 9. After Codex starts the chantier

Hand off to Codex via [`.codex/sdk-prebuilts-prompt.md`](../../.codex/sdk-prebuilts-prompt.md). Codex's M0 spike will exercise popok's new capabilities and produce timing numbers. From then on:

- Watch `gh actions list-runs --repo organic-programming/seed` for the prebuilts workflow.
- Inspect `~/actions-runner/_work/` for build logs and per-target outputs.
- Disk usage on `/Users/popok/.ader-ci-cache/` will grow with vendored builds — no shrinkage policy needed in v1, but watch the trend.

Issues that warrant pausing the chantier and revisiting popok config:

- A target consistently fails with "out of memory" → bump Colima `--memory` or Tart memory.
- Concurrent builds collide on Docker buildx cache → split into per-runner cache dirs.
- Windows VM crashes or fails to boot → re-pull the Tart image, or fall back to Path B (`windows-latest` GitHub-hosted).

---

## 10. Estimated time to complete this runbook

| Section | Time |
|---|---|
| 1. Disk audit | 5 min |
| 2. Colima install + multi-arch | 30 min |
| 3. Tart install + Windows VM pull | 1-2 hours (Windows image is ~30 GB) |
| 3. Windows VM build-tool validation | 30 min |
| 4. Build tools install | 15 min |
| 5. Runner label re-registration | 15 min |
| 6. Network and security verification | 15 min |
| 7. Hygiene cron/launchd setup | 30 min |
| 8. Smoke validation | 30 min |
| **Total** | **~4 hours active work** |

Most of the wall-clock cost is the Windows VM image pull. Run sections 1, 2, 4, 5, 6 while the Tart pull progresses in the background.
