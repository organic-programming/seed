# Install op — the Organic Programming CLI
#
# Usage:
#   irm https://raw.githubusercontent.com/organic-programming/grace-op/dev/scripts/install.ps1 | iex
#
# Flow:
#   1. Download op.exe to a temp directory
#   2. Use temp op to run: op env --init (creates OPPATH, OPBIN, cache)
#   3. Copy binary from temp to OPBIN
#   4. Set OPPATH, OPBIN, PATH persistently (user-level)
#   5. Clean up temp
#
# Respects OPPATH and OPBIN if already set.

$ErrorActionPreference = "Stop"

$Repo = "organic-programming/grace-op"

# ── Detect architecture ──────────────────────────────────────

$Arch = if ([System.Environment]::Is64BitOperatingSystem) {
    if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") { "arm64" } else { "amd64" }
} else {
    Write-Error "32-bit systems are not supported."
    exit 1
}

# ── Download to temp ─────────────────────────────────────────

$TmpDir = Join-Path ([System.IO.Path]::GetTempPath()) "op-install-$([System.Guid]::NewGuid().ToString('N').Substring(0,8))"
New-Item -ItemType Directory -Path $TmpDir -Force | Out-Null

$TmpOp = Join-Path $TmpDir "op.exe"
$ReleaseUrl = "https://github.com/$Repo/releases/latest/download/op_windows_$Arch.exe"

Write-Host "-> Installing op for windows/$Arch..."

try {
    Invoke-WebRequest -Uri $ReleaseUrl -OutFile $TmpOp -UseBasicParsing
    Write-Host "OK Downloaded op"
} catch {
    Write-Host "   No pre-built binary, building via go install..."

    if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
        Write-Error @"
No pre-built binary and Go is not installed.
Install Go from https://go.dev/dl/ or download op from:
https://github.com/$Repo/releases
"@
        exit 1
    }

    $env:GOBIN = $TmpDir
    go install "github.com/$Repo/cmd/op@latest"
    Write-Host "OK Built op"
}

# ── Let op set up its own environment ────────────────────────

& $TmpOp env --init

# Read OPBIN from op.
$EnvOutput = & $TmpOp env 2>$null
$OpPath = ($EnvOutput | Where-Object { $_ -match "^OPPATH=" }) -replace "^OPPATH=", ""
$OpBin  = ($EnvOutput | Where-Object { $_ -match "^OPBIN=" })  -replace "^OPBIN=", ""

Write-Host "   OPBIN = $OpBin"

# ── Install to OPBIN ────────────────────────────────────────

Copy-Item -Path $TmpOp -Destination (Join-Path $OpBin "op.exe") -Force
Write-Host "OK Installed op to $OpBin\op.exe"

# ── Persistent environment (user-level) ──────────────────────

if ($OpPath) {
    [System.Environment]::SetEnvironmentVariable("OPPATH", $OpPath, "User")
}
if ($OpBin) {
    [System.Environment]::SetEnvironmentVariable("OPBIN", $OpBin, "User")

    $CurrentPath = [System.Environment]::GetEnvironmentVariable("PATH", "User")
    if ($CurrentPath -notlike "*$OpBin*") {
        [System.Environment]::SetEnvironmentVariable("PATH", "$OpBin;$CurrentPath", "User")
        Write-Host "OK Added $OpBin to user PATH"
    } else {
        Write-Host "OK $OpBin already in PATH"
    }

    # Also set for current session.
    $env:OPPATH = $OpPath
    $env:OPBIN = $OpBin
    if ($env:PATH -notlike "*$OpBin*") {
        $env:PATH = "$OpBin;$env:PATH"
    }
}

# ── Clean up temp ────────────────────────────────────────────

Remove-Item -Recurse -Force $TmpDir -ErrorAction SilentlyContinue

# ── Done ─────────────────────────────────────────────────────

Write-Host ""
Write-Host "OK Done. Restart your terminal, then run:"
Write-Host "  op version"
Write-Host ""
Write-Host "Or try now (env is set for this session):"
Write-Host "  op version"
