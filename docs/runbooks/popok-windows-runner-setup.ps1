# popok-windows-runner-setup.ps1
#
# Idempotent setup script for a Windows mini-PC to host the SDK prebuilts
# CI matrix's Windows target (x86_64-pc-windows-msvc).
#
# Companion to: docs/runbooks/popok-prebuilts-setup.md (popok-side, macOS)
#
# Usage (PowerShell as Administrator):
#   .\popok-windows-runner-setup.ps1 [-RunnerToken <token>] [-RunnerName <name>]
#
# Prerequisites:
#   - Windows 10 or 11 (x64 or ARM64)
#   - Run as Administrator
#   - Network access (downloads ~10 GB total: Visual Studio Build Tools + dependencies)
#   - At least 100 GB free disk space
#   - An admin user to host the runner service (typically the user running this script)
#
# What this script does:
#   - Verifies Windows version, admin rights, free disk
#   - Installs Chocolatey package manager
#   - Installs Visual Studio Build Tools 2022 with the C++ workload (~10 GB)
#   - Installs Git, CMake, Ninja, 7-Zip via Chocolatey
#   - Downloads the GitHub Actions runner
#   - Optionally configures the runner if a token is provided
#
# What this script does NOT do (manual):
#   - Install Windows itself
#   - Configure user accounts
#   - Configure firewall rules beyond default
#   - Install the runner as a service if no token provided (use --runasservice with config.cmd)

[CmdletBinding()]
param(
  [string]$RunnerToken = "",
  [string]$RunnerName = "popok-windows",
  [string]$RunnerLabels = "self-hosted,popok-windows,windows-vm",
  [string]$RepoUrl = "https://github.com/organic-programming/seed",
  [string]$RunnerVersion = "2.334.0",
  [string]$InstallRoot = "C:\actions-runner"
)

$ErrorActionPreference = "Stop"

# ──────────────────────────────────────────────────────────────────────────────
# Logging helpers
# ──────────────────────────────────────────────────────────────────────────────

function Log-Info { param($msg) Write-Host "[setup] $msg" -ForegroundColor Cyan }
function Log-Ok   { param($msg) Write-Host "[ ok ] $msg" -ForegroundColor Green }
function Log-Warn { param($msg) Write-Warning "[warn] $msg" }
function Log-Fail { param($msg) Write-Host "[fail] $msg" -ForegroundColor Red; exit 1 }

# ──────────────────────────────────────────────────────────────────────────────
# 1. Preflight
# ──────────────────────────────────────────────────────────────────────────────

function Preflight {
  Log-Info "Preflight checks..."

  # Admin
  $isAdmin = ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
  if (-not $isAdmin) { Log-Fail "This script must run as Administrator." }
  Log-Ok "Running as Administrator"

  # Windows version
  $os = Get-CimInstance Win32_OperatingSystem
  $version = [Version]$os.Version
  if ($version.Major -lt 10) { Log-Fail "Windows 10 or later required (got $($os.Caption))" }
  Log-Ok "Windows: $($os.Caption) ($($os.Version), $($os.OSArchitecture))"

  # Free disk
  $drive = (Get-PSDrive C)
  $freeGB = [math]::Round($drive.Free / 1GB)
  if ($freeGB -lt 100) {
    Log-Warn "Free disk space on C: is $freeGB GB; at least 100 GB recommended."
    $confirm = Read-Host "Continue anyway? [y/N]"
    if ($confirm -ne "y" -and $confirm -ne "Y") { Log-Fail "Aborted by user" }
  } else {
    Log-Ok "Free disk space on C: $freeGB GB"
  }
}

# ──────────────────────────────────────────────────────────────────────────────
# 2. Chocolatey
# ──────────────────────────────────────────────────────────────────────────────

function Install-Chocolatey {
  Log-Info "Checking Chocolatey..."
  if (Get-Command choco -ErrorAction SilentlyContinue) {
    Log-Ok "Chocolatey already installed"
    return
  }
  Log-Info "Installing Chocolatey..."
  Set-ExecutionPolicy Bypass -Scope Process -Force
  [System.Net.ServicePointManager]::SecurityProtocol = [System.Net.ServicePointManager]::SecurityProtocol -bor 3072
  Invoke-Expression ((New-Object System.Net.WebClient).DownloadString('https://community.chocolatey.org/install.ps1'))
  $env:Path = [System.Environment]::GetEnvironmentVariable("Path","Machine") + ";" + [System.Environment]::GetEnvironmentVariable("Path","User")
  Log-Ok "Chocolatey installed"
}

# ──────────────────────────────────────────────────────────────────────────────
# 3. Visual Studio Build Tools 2022 with C++ workload
# ──────────────────────────────────────────────────────────────────────────────

function Install-BuildTools {
  Log-Info "Checking Visual Studio Build Tools 2022..."

  $vswhere = "${env:ProgramFiles(x86)}\Microsoft Visual Studio\Installer\vswhere.exe"
  if (Test-Path $vswhere) {
    $installed = & $vswhere -products Microsoft.VisualStudio.Product.BuildTools -property installationPath 2>$null
    if ($installed) {
      Log-Ok "Build Tools 2022 already installed at: $installed"
      return
    }
  }

  Log-Info "Installing Visual Studio Build Tools 2022 with C++ workload (~10 GB, 20-40 min)..."
  choco install -y visualstudio2022buildtools `
    --package-parameters="--add Microsoft.VisualStudio.Workload.VCTools --add Microsoft.VisualStudio.Component.VC.CMake.Project --add Microsoft.VisualStudio.Component.Windows11SDK.22621 --includeRecommended --quiet --wait"
  Log-Ok "Visual Studio Build Tools 2022 installed"
}

# ──────────────────────────────────────────────────────────────────────────────
# 4. Companion tools (Git, CMake standalone, Ninja, 7-Zip)
# ──────────────────────────────────────────────────────────────────────────────

function Install-CompanionTools {
  Log-Info "Installing Git, CMake, Ninja, 7-Zip via Chocolatey..."

  $packages = @("git", "cmake", "ninja", "7zip")
  foreach ($pkg in $packages) {
    if (choco list --local-only --exact $pkg -r) {
      Log-Ok "Already installed: $pkg"
    } else {
      Log-Info "Installing: $pkg"
      choco install -y $pkg
    }
  }

  # Refresh PATH
  $env:Path = [System.Environment]::GetEnvironmentVariable("Path","Machine") + ";" + [System.Environment]::GetEnvironmentVariable("Path","User")
}

# ──────────────────────────────────────────────────────────────────────────────
# 5. GitHub Actions runner download + optional config
# ──────────────────────────────────────────────────────────────────────────────

function Install-Runner {
  Log-Info "Setting up GitHub Actions runner at $InstallRoot..."

  if (-not (Test-Path $InstallRoot)) {
    New-Item -ItemType Directory -Path $InstallRoot | Out-Null
  }

  $runnerZip = "$InstallRoot\actions-runner-win-x64-$RunnerVersion.zip"
  $runnerExe = "$InstallRoot\config.cmd"

  if (Test-Path $runnerExe) {
    Log-Ok "Runner already extracted at $InstallRoot"
  } else {
    if (-not (Test-Path $runnerZip)) {
      Log-Info "Downloading runner v$RunnerVersion..."
      $arch = if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") { "arm64" } else { "x64" }
      $url = "https://github.com/actions/runner/releases/download/v$RunnerVersion/actions-runner-win-$arch-$RunnerVersion.zip"
      Invoke-WebRequest -Uri $url -OutFile $runnerZip -UseBasicParsing
    }
    Log-Info "Extracting runner..."
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    [System.IO.Compression.ZipFile]::ExtractToDirectory($runnerZip, $InstallRoot)
    Log-Ok "Runner extracted"
  }

  # Configure if token provided
  if ($RunnerToken -ne "") {
    Log-Info "Configuring runner with name='$RunnerName' labels='$RunnerLabels'..."
    Push-Location $InstallRoot
    try {
      .\config.cmd --url $RepoUrl --token $RunnerToken --name $RunnerName --labels $RunnerLabels --work _work --replace --runasservice --unattended
      Log-Ok "Runner configured and installed as a Windows service"
    } finally {
      Pop-Location
    }
  } else {
    Log-Warn "No -RunnerToken provided. Runner downloaded but not configured."
    Log-Warn "To finish: get a token at $RepoUrl/settings/actions/runners/new"
    Log-Warn "Then run: cd $InstallRoot ; .\config.cmd --url $RepoUrl --token <TOKEN> --name $RunnerName --labels $RunnerLabels --work _work --replace --runasservice --unattended"
  }
}

# ──────────────────────────────────────────────────────────────────────────────
# 6. Summary
# ──────────────────────────────────────────────────────────────────────────────

function Print-Summary {
  Write-Host ""
  Write-Host "==============================================================================" -ForegroundColor Green
  Write-Host "Windows runner setup complete (automated portion)" -ForegroundColor Green
  Write-Host "==============================================================================" -ForegroundColor Green
  Write-Host ""
  Write-Host "Installed and configured:"
  Write-Host "  ✓ Chocolatey"
  Write-Host "  ✓ Visual Studio Build Tools 2022 (C++ workload + Windows SDK)"
  Write-Host "  ✓ Git, CMake, Ninja, 7-Zip"
  Write-Host "  ✓ GitHub Actions runner v$RunnerVersion at $InstallRoot"
  if ($RunnerToken -ne "") {
    Write-Host "  ✓ Runner registered as service with labels: $RunnerLabels"
  } else {
    Write-Host "  ⚠ Runner NOT yet registered (no -RunnerToken passed)"
  }
  Write-Host ""
  Write-Host "Sanity checks:"
  Write-Host "  cl.exe        # MSVC C/C++ compiler"
  Write-Host "  cmake --version"
  Write-Host "  ninja --version"
  Write-Host "  git --version"
  Write-Host ""
  if ($RunnerToken -eq "") {
    Write-Host "Final manual step — register the runner:"
    Write-Host "  1. Get token from: $RepoUrl/settings/actions/runners/new"
    Write-Host "  2. Run:"
    Write-Host "     cd $InstallRoot"
    Write-Host "     .\config.cmd --url $RepoUrl --token <TOKEN> --name $RunnerName --labels $RunnerLabels --work _work --replace --runasservice --unattended"
    Write-Host ""
  }
  Write-Host "==============================================================================" -ForegroundColor Green
}

# ──────────────────────────────────────────────────────────────────────────────
# Main
# ──────────────────────────────────────────────────────────────────────────────

Log-Info "Starting Windows runner setup..."

Preflight
Install-Chocolatey
Install-BuildTools
Install-CompanionTools
Install-Runner
Print-Summary

Log-Ok "Done."
