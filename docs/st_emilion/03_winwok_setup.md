# winwok preparation — SDK prebuilts Windows runner

Status: runbook  
Date: 2026-04-26  
References: [`02_popok_setup.md`](02_popok_setup.md), [`docs/specs/sdk-prebuilts.md`](../specs/sdk-prebuilts.md)

This runbook prepares `winwok` (the Windows mini-PC) to host the `x86_64-pc-windows-msvc` target of the SDK prebuilts CI matrix. Companion to [`02_popok_setup.md`](02_popok_setup.md) which sets up macOS + Linux targets on popok.

`winwok` is a Minisforum basic NUC running Windows 11 Pro, deployed alongside popok at Saint-Émilion on residential fibre Free, accessed via WireGuard on the Freebox.

The automated portion is in [`winwok-runner-setup.ps1`](winwok-runner-setup.ps1). This document covers the preparation that script cannot automate and the verification afterwards.

---

## 1. Hardware inventory and prerequisites

| Item | Recommendation |
|---|---|
| CPU | 4 cores minimum, 8 cores comfortable |
| RAM | 8 GB minimum, 16 GB comfortable |
| Storage | 256 GB SSD minimum, 100 GB free required |
| Network | Wired Ethernet preferred over WiFi for CI stability |
| OS | Windows 11 Pro (clean install, see §3) |

Modest hardware suffices because the prebuilts workload is rare (3 builds per release × monthly cadence ≈ 3-4 builds per month). Each build takes ~30-60 min wall time on a 4-core machine.

---

## 2. Pre-deployment checklist (Mon-Wed before Thursday's installation)

These items can be done in advance of the Saint-Émilion trip:

- Download Windows 11 Pro ISO from https://www.microsoft.com/software-download/windows11.
- Create a bootable USB key (16+ GB, USB 3) using Rufus on Windows or the Microsoft Media Creation Tool.
- Verify the Minisforum boots from USB by testing once at home (BIOS access typically F7 or F11 at boot).
- Note the BIOS/UEFI version in case driver updates are needed.
- Pack: Minisforum + power adapter + USB key + spare HDMI cable + USB keyboard/mouse + Ethernet cable.

---

## 3. Clean Windows install (recommended)

A clean install eliminates pre-installed bloatware and gives a known-trustworthy baseline for a CI runner that will hold a GitHub Actions token.

### 3.1 Boot from USB

- Plug the bootable USB key.
- Power on while pressing the boot menu key (F7 or F11 on most Minisforum models).
- Select the USB key.

### 3.2 Custom install

- Choose **Custom install**, not Upgrade.
- Wipe all existing partitions on the internal SSD. Repartition fresh.
- Continue through the language and keyboard prompts.

### 3.3 Skip Microsoft Account

A CI runner needs a local admin account, not a Microsoft Account.

- At the "Sign in" screen, press `Shift+F10` to open Command Prompt.
- Type `oobe\BypassNRO` and press Enter; the machine reboots.
- After reboot, you can choose "I don't have internet" → "Continue with limited setup" → create a local account.
- Username suggestion: `winwok-admin`. Strong password.

### 3.4 Apply updates

After the desktop appears:

- Settings → Windows Update → Check for updates → install all available.
- Reboot when prompted. May take 30+ min and 1-2 reboots.

### 3.5 Install drivers from manufacturer

- From a separate machine, download chipset / network / GPU drivers from the official Minisforum website for the exact model.
- Transfer via USB key.
- Install only what's needed (typically network if Ethernet doesn't work out-of-the-box).

### 3.6 Disable bloat (optional but recommended)

- Settings → Privacy → Telemetry → Required diagnostic data only.
- Settings → Apps → uninstall OneDrive, Teams, Office trial, anything not needed.
- Settings → Personalization → Background → static color (saves CPU during builds).

---

## 4. Run the automated setup

Once Windows is clean and on the network, the PowerShell script does the rest.

### 4.1 Open PowerShell as Administrator

- Start menu → search "PowerShell" → right-click → **Run as Administrator**.

### 4.2 Get the script

Two options:

**Option A** — clone the seed repo:
```powershell
# If git is not yet installed, install it first via the next command
# Then:
git clone -b dev https://github.com/organic-programming/seed.git C:\seed
cd C:\seed
.\docs\st_emilion\winwok-runner-setup.ps1
```

**Option B** — download just the script:
```powershell
$url = "https://raw.githubusercontent.com/organic-programming/seed/dev/docs/st_emilion/winwok-runner-setup.ps1"
Invoke-WebRequest -Uri $url -OutFile $env:TEMP\winwok-runner-setup.ps1
& $env:TEMP\winwok-runner-setup.ps1
```

### 4.3 What the script does

Per [`winwok-runner-setup.ps1`](winwok-runner-setup.ps1):

- Verifies Windows 10+ on x64 or ARM64, admin rights, free disk.
- Installs Chocolatey package manager.
- Installs **Visual Studio Build Tools 2022** with the C++ workload + Windows 11 SDK (~10 GB, 20-40 min).
- Installs Git, CMake, Ninja, 7-Zip via Chocolatey.
- Downloads the GitHub Actions runner v2.334.0 to `C:\actions-runner`.
- If a `-RunnerToken` is passed, configures the runner as a Windows service. Otherwise, prints the configuration command for you to run after fetching a token from GitHub UI.

### 4.4 Wall time estimate

- Chocolatey install: < 5 min.
- Visual Studio Build Tools download + install: 20-40 min depending on network.
- Companion tools (Git, CMake, Ninja, 7-Zip): < 5 min.
- Runner download + extraction: < 2 min.
- **Total**: 30-60 min, mostly waiting on Microsoft's CDN.

The script is idempotent — safe to re-run if interrupted.

---

## 5. Register the runner with GitHub

The script downloads the runner but cannot register it without a token. Tokens come from GitHub UI and expire in ~1 hour, so do this step right after the script completes.

### 5.1 Get a registration token

- Open https://github.com/organic-programming/seed/settings/actions/runners/new in a browser.
- Choose Windows / x64 (or ARM64 if applicable).
- Copy the token from the `--token` argument shown.

### 5.2 Register

```powershell
cd C:\actions-runner
.\config.cmd `
    --url https://github.com/organic-programming/seed `
    --token <TOKEN_FROM_GITHUB_UI> `
    --name winwok `
    --labels self-hosted,winwok,windows `
    --work _work `
    --replace `
    --runasservice `
    --unattended
```

The `--runasservice` flag installs the runner as a Windows service that auto-starts on boot. `--unattended` suppresses interactive prompts.

### 5.3 Verify the runner is online

- Check service status:
  ```powershell
  Get-Service "actions.runner.organic-programming-seed.winwok"
  ```
  Expected: `Running`.

- Check from GitHub UI: https://github.com/organic-programming/seed/settings/actions/runners. `winwok` should show `Idle` (green).

- From any machine via API:
  ```bash
  gh api repos/organic-programming/seed/actions/runners --jq '.runners[] | select(.name=="winwok") | {status, labels: [.labels[].name]}'
  ```
  Expected: `{"status": "online", "labels": ["self-hosted", "winwok", "windows"]}`.

---

## 6. Sanity check the build environment

Before signing off, run a smoke build to confirm winwok can actually compile a non-trivial C++ project against MSVC.

```powershell
# Open a new PowerShell as winwok-admin (not necessarily Administrator)
mkdir C:\winwok-smoke
cd C:\winwok-smoke

# Clone a small CMake-based C++ project
git clone --depth 1 https://github.com/Microsoft/vcpkg-tool.git
cd vcpkg-tool

# Configure with the MSVC toolchain (the generator picks it up via vswhere)
cmake -B build -G "Visual Studio 17 2022" -A x64
cmake --build build --config Release
```

If this completes without errors, MSVC is correctly installed and discoverable. The actual prebuilts builds will use a similar chain.

Clean up:
```powershell
cd C:\
Remove-Item -Recurse -Force C:\winwok-smoke
```

---

## 7. Network and remote admin

WireGuard for remote admin is configured on the Freebox per [`01_runners.md`](01_runners.md) §4.4. From the laptop, you can SSH or RDP into winwok over the VPN tunnel.

### 7.1 Enable RDP on winwok

If you want graphical remote admin:

- Settings → System → Remote Desktop → Enable.
- Note: Windows 11 Home does NOT support RDP; Pro is required (already a prerequisite per §1).
- Allow RDP through the firewall: should be automatic when enabling the toggle.

From the laptop (over WireGuard):
```bash
# macOS Microsoft Remote Desktop app, or any RDP client
# Connect to winwok's LAN IP (e.g., 192.168.1.42)
```

### 7.2 Enable OpenSSH (optional, lighter weight)

- Settings → Apps → Optional features → Add a feature → search "OpenSSH Server" → install.
- Start the service:
  ```powershell
  Start-Service sshd
  Set-Service -Name sshd -StartupType Automatic
  ```

From the laptop:
```bash
ssh winwok-admin@<winwok-LAN-IP>
```

---

## 8. Operational hygiene

### 8.1 Power management

- Settings → System → Power → Screen and sleep → Plugged in: Never.
- Settings → System → Power → Lid and power button settings: Do nothing on lid close (if applicable).

A CI runner must not sleep, hibernate, or shut down opportunistically.

### 8.2 Windows Update policy

Default Windows Update is acceptable but reboots interrupt CI jobs. Two options:

- **Defer reboots**: Settings → Windows Update → Pause updates → up to 5 weeks.
- **Active hours**: Settings → Windows Update → Advanced options → Active hours: 24h. Won't reboot during "active" hours.

For a residential CI runner, manually applying updates monthly is acceptable.

### 8.3 Disk-space monitoring

The runner accumulates build artifacts under `C:\actions-runner\_work\`. Periodic cleanup:

- After each major release: `Remove-Item -Recurse C:\actions-runner\_work\<repo>\*\.zig-vendor` (or equivalent) to drop vendored caches.
- Or: schedule a Task Scheduler job to clean weekly (mirroring the launchd job on popok).

---

## 9. Smoke validation — end-to-end

Once everything above is done, run this sequence to confirm winwok is fully operational:

```powershell
# Tools all present
foreach ($cmd in @("cl", "cmake", "ninja", "git")) {
    Get-Command $cmd -ErrorAction SilentlyContinue | Select-Object Name, Path
}

# Runner service running
Get-Service "actions.runner.*winwok"

# Trigger a no-op job from the laptop
gh workflow run --field runner=winwok ...    # if such a workflow exists
```

Expected: all four tools resolve, the runner service is `Running`, and the no-op job picks up on winwok.

If all three pass, winwok is ready.

---

## 10. Estimated time

| Section | Time |
|---|---|
| 2. Pre-deployment checklist | 30 min (off-site) |
| 3. Clean Windows install | 1.5-2 hours (mostly waiting) |
| 4. Run automated setup | 30-60 min |
| 5. Register runner with GitHub | 5 min |
| 6. Sanity-check build | 10 min |
| 7. Network and remote admin | 15 min |
| 8. Operational hygiene | 15 min |
| 9. Smoke validation | 5 min |
| **Total** | **~3-4 hours active** |

Most wall-clock cost is the Visual Studio Build Tools install. Run sections 5, 7, 8 in parallel while §4 progresses.
