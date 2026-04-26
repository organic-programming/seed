# Saint-Émilion runners — deployment plan

Status: plan  
Date: 2026-04-26 (Monday)  
Deployment date: Thursday 2026-04-30  
Site: Saint-Émilion, residential fiber Free, accessed via WireGuard on the Freebox

This document captures the plan for hosting the SDK prebuilts CI runners (`popok` and `winwok`) on residential fiber at Saint-Émilion. Until the on-site setup is complete (Thursday), the prebuilts chantier — when it kicks off — will use GitHub-hosted runners as a transitional fallback.

---

## 1. Decision

**Until Thursday 2026-04-30**: any prebuilts work that needs CI uses GitHub-hosted runners (`macos-14`, `ubuntu-latest`, `ubuntu-24.04-arm`, `windows-latest`). Acceptable cost (it's a public repo, GitHub-hosted is free) and avoids coupling the chantier kickoff to physical infra readiness.

**After Thursday**: popok hosts macOS + Linux targets, winwok hosts the Windows target. Path A across all 7 targets per [`docs/specs/sdk-prebuilts.md`](specs/sdk-prebuilts.md) §6.2.

The transition (GitHub-hosted → self-hosted) lands as a follow-up PR after both runners are operational and have passed a smoke build.

---

## 2. Inventory

### Hardware

| Machine | Role | Status (Monday) | Specs |
|---|---|---|---|
| **popok** | macOS native + Linux via Docker (6 targets) | Set up at current location, will be moved Thursday | Apple Silicon Mac, sufficient cores/RAM/disk, see audit |
| **winwok** | Windows MSVC native (1 target) | Minisforum basic NUC, **not yet provisioned** | x86_64 mini-PC, modern Intel/AMD, 8+ GB RAM, 256+ GB SSD |

### Network

| Item | Detail |
|---|---|
| ISP | Free Fibre |
| Router | Freebox |
| Public IPv4 | Default on Free fiber unless CGNAT explicitly opted in (verify Thursday) |
| WireGuard | Built into Freebox (Settings → VPN), used for remote admin only |
| LAN topology | Both runners on same wired LAN (Ethernet preferred over WiFi for stability) |

### Software ready to deploy

- `docs/runbooks/popok-prebuilts-setup.sh` — already run on popok at current location, idempotent.
- `docs/runbooks/winwok-runner-setup.ps1` — ready to run on the Minisforum after Windows is installed.
- `.codex/sdk-prebuilts-prompt.md` — Codex chantier prompt, awaiting both runners + Zig P12 merge.

---

## 3. Pre-Thursday checklist (Mon–Wed)

Things to do or verify **before** travelling to Saint-Émilion. None are blocking individually, but the more is done in advance, the smoother Thursday goes.

| Task | Notes | Owner |
|---|---|---|
| Download Windows 11 Pro ISO from Microsoft | https://www.microsoft.com/software-download/windows11. Can do from any machine. | composer |
| Create bootable USB key (16+ GB, USB 3) | Use Rufus on Windows, or `dd` from macOS, or Microsoft Media Creation Tool. Test boots into Windows installer. | composer |
| Verify Minisforum boots (current state) | Plug it in once at home, confirm BIOS access (typically F7 or F11 at boot for boot menu, DEL or F2 for BIOS setup). Note BIOS/UEFI version. | composer |
| Pack: Minisforum + power adapter + USB key + HDMI cable + USB keyboard/mouse + Ethernet cable | Most Minisforum models include the cable in the box. Spare Ethernet cable for the Freebox patch panel just in case. | composer |
| Save current popok config snapshot | `brew list > ~/popok-brew-state-pre-move.txt` and `tart list > ~/popok-tart-state-pre-move.txt` for reference, in case anything breaks after the move. | composer |
| Confirm GitHub repo settings | Settings → Actions → General → "Fork pull request workflows" set to require approval for first-time contributors. Pre-emptive hardening for self-hosted runners. | composer |
| Have a GitHub admin token at hand | Standard personal access token with `repo` scope, used on Thursday to register both runners. Tokens for runner registration come from the Settings → Actions → Runners page (per-repo, expire ~1h, fetch on Thursday). | composer |

---

## 4. Thursday plan — hour-by-hour

Estimates; reorder as conditions allow on the day.

### 09:00–10:00 — Site arrival, network verification

- Connect laptop to the Saint-Émilion LAN (Ethernet).
- Verify outbound: `curl -I https://github.com`, `ping -c 3 ghcr.io`. Both should succeed.
- Check Freebox admin: log in to https://mafreebox.freebox.fr/ (or the Freebox public hostname).
- Verify the Freebox has a **public IPv4** assigned (Settings → Internet → IPv4 type ≠ CGNAT). If CGNAT, escalate to Free or accept that WireGuard-from-outside won't work without an IPv6 fallback.
- Note the public IPv4 (or DDNS hostname) for later.

### 10:00–11:00 — popok physical install

- Place popok in its target spot (proximity to Freebox, ventilation, power).
- Connect Ethernet, power on.
- Verify `ssh popok` from the laptop.
- Confirm services restored: `colima status`, `tart list`, `~/code/actions-runner/run.sh --check` (or whatever the existing GitHub Actions runner status command is — `./svc.sh status`).
- Confirm the Actions Runner reconnects to GitHub: visit https://github.com/organic-programming/seed/settings/actions/runners. `popok` should appear as `Idle` (green). If not, restart the runner service: `cd ~/code/actions-runner; sudo ./svc.sh stop; sudo ./svc.sh start`.

### 11:00–12:30 — winwok Windows install

- Plug Minisforum: HDMI to a temporary monitor, USB keyboard, USB key (Windows installer), Ethernet, power.
- Boot from USB (F7 or F11 at boot).
- Install Windows 11 Pro:
  - **Custom install**, wipe all existing partitions.
  - Skip Microsoft Account: at the "Sign in" screen, `Shift+F10` → cmd → `oobe\BypassNRO` → reboot → use local account.
  - Local admin user, name something like `winwok-admin`, strong password.
- Apply Windows Updates (will need a reboot or two, ~30 min).
- Install only chipset/network drivers from the Minisforum official site. Skip everything else.

### 12:30–13:30 — Lunch break, let Windows Updates finish

### 13:30–14:30 — winwok runner setup

- Open PowerShell **as Administrator** on winwok (RDP if you've enabled it, or stay local with monitor).
- Clone the seed repo:
  ```powershell
  git clone -b dev https://github.com/organic-programming/seed.git C:\seed
  cd C:\seed
  ```
  (Git for Windows installer if not already done — but the runbook script will install Git via Chocolatey, so a `git clone` may need to wait for after the script. Alternative: download just the script via `Invoke-WebRequest` and run.)
- Run the setup:
  ```powershell
  Invoke-WebRequest -Uri https://raw.githubusercontent.com/organic-programming/seed/dev/docs/runbooks/winwok-runner-setup.ps1 -OutFile $env:TEMP\winwok-runner-setup.ps1
  & $env:TEMP\winwok-runner-setup.ps1
  ```
- Wait for Visual Studio Build Tools install (~30-45 min). Go for a walk.

### 14:30–15:00 — winwok runner registration

- Get a registration token: https://github.com/organic-programming/seed/settings/actions/runners/new.
- On winwok PowerShell:
  ```powershell
  cd C:\actions-runner
  .\config.cmd --url https://github.com/organic-programming/seed --token <TOKEN> --name winwok --labels self-hosted,winwok,windows --work _work --replace --runasservice --unattended
  ```
- Verify on https://github.com/organic-programming/seed/settings/actions/runners that `winwok` is `Idle` (green).

### 15:00–15:30 — popok runner re-registration with new labels

If popok's labels are still the original (`self-hosted, popok` only), re-register with the granular labels per [`docs/runbooks/popok-prebuilts-setup.md`](runbooks/popok-prebuilts-setup.md) §5:

```bash
ssh popok
cd /Users/popok/code/actions-runner
sudo ./svc.sh stop
./config.sh \
  --url https://github.com/organic-programming/seed \
  --token <TOKEN_FROM_GITHUB_UI> \
  --name popok \
  --labels self-hosted,popok,macos,linux-via-docker \
  --work _work \
  --replace
sudo ./svc.sh start
```

(Token for popok = a fresh token from the same Settings → Actions → Runners page.)

### 15:30–16:00 — Freebox WireGuard setup

- Freebox admin → Paramètres → VPN → WireGuard → enable.
- Generate server keypair, generate client keypair for the laptop.
- Note the WireGuard endpoint (`<public-ip>:51820` or whatever port).
- On the laptop, install WireGuard client, import the generated config.
- Test: turn off WiFi, enable LTE/4G, activate the WireGuard tunnel, `ssh popok` should still work.

### 16:00–17:00 — Smoke validation

- On laptop (back through WireGuard or local LAN, doesn't matter for validation):
  ```bash
  # Trigger a no-op job that exercises each runner label
  gh workflow run --field runner=popok ...    # if such a workflow exists
  ```
  Or simply check via API:
  ```bash
  gh api repos/organic-programming/seed/actions/runners --jq '.runners[] | {name, status, labels: [.labels[].name]}'
  ```
- Expected output:
  ```json
  {"name": "popok",  "status": "online", "labels": ["self-hosted", "popok",  "macos", "linux-via-docker"]}
  {"name": "winwok", "status": "online", "labels": ["self-hosted", "winwok", "windows"]}
  ```

### 17:00 — End of day

Both runners online with correct labels. Optionally run a real build to test:

```bash
op build gabriel-greeting-go --target linux/amd64 --runner popok    # if such a flag exists
```

Or wait for the next CI run from a real PR (Zig P12 might still be in flight).

---

## 5. Transition: GitHub-hosted → self-hosted

After Thursday, both runners are operational. The Codex prebuilts chantier (when it kicks off after Zig P12 merges) initially has its workflows pointing to GitHub-hosted runners (per the "first builds use GitHub instances" decision in §1). The transition is a follow-up PR:

```yaml
# Before (GitHub-hosted, transitional):
runs-on: macos-14
runs-on: ubuntu-latest
runs-on: ubuntu-24.04-arm
runs-on: windows-latest

# After (self-hosted, target state):
runs-on: [self-hosted, popok, macos]
runs-on: [self-hosted, popok, linux-via-docker]   # with container: linux/amd64 or linux/arm64
runs-on: [self-hosted, winwok, windows]
```

The transition PR is small (workflow YAML edits only) and lands when both runners have passed at least one full prebuilts release cycle on GitHub-hosted. That gives confidence that the workflow logic itself works before adding the self-hosted layer.

---

## 6. Risks and contingencies

| Risk | Probability | Mitigation |
|---|---|---|
| Saint-Émilion fiber down on Thursday | Low | Test connectivity on arrival before doing anything else. If down, defer the install, the chantier stays on GitHub-hosted. |
| Free has CGNAT (no public IPv4) | Low (Free defaults to public IPv4) | WireGuard inbound won't work. Either escalate with Free, use the Freebox's IPv6 (most ISPs give /48), or fall back to remote admin via Tailscale (no port forwarding needed). |
| Minisforum NIC needs proprietary driver | Medium | Carry the proprietary driver pack on a second USB key. Also have a USB-Ethernet dongle as backup. |
| Visual Studio Build Tools install fails | Low | Re-run the script (idempotent). If still fails, install VS Build Tools manually via the GUI installer from microsoft.com. |
| popok's existing actions-runner doesn't reconnect after move | Medium | Restart the service. If still broken, re-register with the new token (the script supports `--replace`). |
| Power outage during install | Low | Each step is restartable. The clean install is the most fragile — on power loss mid-install, redo from BIOS boot menu. |
| WireGuard on Freebox doesn't work as expected | Medium | Tailscale is a 30-min fallback (peer-to-peer, no port forwarding). Install on both popok and winwok (and the laptop) and you're done. |

---

## 7. Definition of done (Thursday end-of-day)

- [ ] popok physically deployed at Saint-Émilion, online, runner registered with labels `self-hosted, popok, macos, linux-via-docker`.
- [ ] winwok physically deployed, Windows 11 Pro clean install, runner registered with labels `self-hosted, winwok, windows`.
- [ ] Both runners visible as `Idle (online)` on https://github.com/organic-programming/seed/settings/actions/runners.
- [ ] WireGuard (or Tailscale) configured for remote admin from the laptop.
- [ ] Confirmation that the Saint-Émilion fiber sustains the workload (run a manual `colima status` + a full Docker pull as a smoke).

If any of these is not done, the chantier stays on GitHub-hosted runners and the self-hosted transition lands later.

---

## 8. After Thursday

- File a tracking note that the prebuilts chantier can transition to self-hosted runners.
- When Codex's Zig P12 chantier merges and the prebuilts chantier kicks off, its initial M0 + Phase 1 PRs use GitHub-hosted runners (workflow YAML).
- Once those phases are stable (all green for at least one full release cycle), open a transition PR swapping `runs-on:` to the self-hosted labels.
- The transition PR can be a 5-minute hand-edit, no coordination with Codex needed.
