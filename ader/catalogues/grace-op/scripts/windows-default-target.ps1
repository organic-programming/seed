#requires -Version 7.0
[CmdletBinding()]
param([Parameter(Mandatory)][ValidateSet('prepare', 'CLI', 'API', 'RPC')][string]$Facet)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
if (-not $IsWindows) { throw 'This suite qualifies native Windows only.' }
$snapshot = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '../../../..'))
if (-not $env:ADER_REPO_ROOT -or -not $env:ADER_RUN_ARTIFACTS -or
    $snapshot -ne [IO.Path]::GetFullPath($env:ADER_REPO_ROOT)) {
    throw 'Run this proof through Ader against its isolated snapshot.'
}
$state = Join-Path $env:ADER_RUN_ARTIFACTS 'default-target'
New-Item -ItemType Directory -Path $state -Force | Out-Null
$bootstrap = (Get-Command op -CommandType Application -ErrorAction Stop | Select-Object -First 1).Source
$env:OPPATH = Join-Path $state 'op-home'
$env:OPBIN = Join-Path $env:OPPATH 'bin'
$env:OPROOT = $snapshot
$env:GRACE_OP_SHARED_CACHE_DIR = Join-Path $state 'package-cache'
$env:ADER_DEFAULT_TARGET_RESULTS = $state
$env:ADER_DEFAULT_TARGET_OP = Join-Path $snapshot 'holons/grace-op/.op/build/grace-op.holon/bin/windows_amd64/op.exe'
Push-Location $snapshot
try {
    if ($Facet -eq 'prepare') {
        # Canonical source build inside the snapshot; never install or replace
        # the caller's OP, and never resolve a live Liz/service workspace.
        & $bootstrap --root $snapshot --no-cache build op
        if ($LASTEXITCODE -ne 0) { throw "Isolated OP build failed: $LASTEXITCODE" }
        if (-not (Test-Path -LiteralPath $env:ADER_DEFAULT_TARGET_OP -PathType Leaf)) { throw 'Isolated OP artifact is missing.' }
        [ordered]@{
            snapshot = $snapshot
            binary = $env:ADER_DEFAULT_TARGET_OP
            sha256 = (Get-FileHash -LiteralPath $env:ADER_DEFAULT_TARGET_OP).Hash
            bootstrapSHA256 = (Get-FileHash -LiteralPath $bootstrap).Hash
        } | ConvertTo-Json | Set-Content -LiteralPath (Join-Path $state 'op-provenance.json') -Encoding utf8
    } else {
        if (-not (Test-Path -LiteralPath $env:ADER_DEFAULT_TARGET_OP -PathType Leaf)) { throw 'Isolated OP preparation did not succeed.' }
        & go test -tags e2e -count=1 -v -timeout 2m "-run=^TestWindowsDefaultTarget_$Facet`$" ./ader/catalogues/grace-op/integration/windows_default_target
        if ($LASTEXITCODE -ne 0) { throw "Default-target $Facet proof failed: $LASTEXITCODE" }
    }
} finally { Pop-Location }
