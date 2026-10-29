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
$state = Join-Path $env:ADER_RUN_ARTIFACTS 'build-environment'
New-Item -ItemType Directory -Path $state -Force | Out-Null
$bootstrap = (Get-Command op -CommandType Application -ErrorAction Stop | Select-Object -First 1).Source
$env:OPPATH = Join-Path $state 'op-home'
$env:OPBIN = Join-Path $env:OPPATH 'bin'
$env:OPROOT = $snapshot
$env:GRACE_OP_SHARED_CACHE_DIR = Join-Path $state 'package-cache'
$env:ADER_BUILD_ENV_RESULTS = $state
$env:ADER_BUILD_ENV_OP = Join-Path $snapshot 'holons/grace-op/.op/build/grace-op.holon/bin/windows_amd64/op.exe'
Push-Location $snapshot
try {
    if ($Facet -eq 'prepare') {
        # Build the frozen source with the installed canonical bootstrap OP;
        # never install it or resolve the live VideoSteno desktop members.
        & $bootstrap --root $snapshot --no-cache build op
        if ($LASTEXITCODE -ne 0) { throw "Isolated OP build failed: $LASTEXITCODE" }
        if (-not (Test-Path -LiteralPath $env:ADER_BUILD_ENV_OP -PathType Leaf)) { throw 'Isolated OP artifact is missing.' }
        $sourceHashes = [ordered]@{}
        foreach ($relative in @('internal/buildenv/environment_windows.go', 'internal/holons/lifecycle.go', 'internal/holons/codegen.go', 'internal/holons/runner_registry.go', 'internal/holons/cargo_target_windows.go')) {
            $sourceHashes[$relative] = (Get-FileHash -LiteralPath (Join-Path $snapshot "holons/grace-op/$relative")).Hash
        }
        $provenance = [ordered]@{
            snapshot = $snapshot
            binary = $env:ADER_BUILD_ENV_OP
            sha256 = (Get-FileHash -LiteralPath $env:ADER_BUILD_ENV_OP).Hash
            bootstrapSHA256 = (Get-FileHash -LiteralPath $bootstrap).Hash
            fixtureSHA256 = (Get-FileHash -LiteralPath (Join-Path $snapshot 'ader/catalogues/grace-op/integration/windows_build_environment/environment_windows_test.go')).Hash
            sourceSHA256 = $sourceHashes
        } | ConvertTo-Json
        $provenance | Set-Content -LiteralPath (Join-Path $state 'op-provenance.json') -Encoding utf8
        # Ader archives step logs; temporary per-run artifacts are cleaned.
        Write-Output $provenance
    } else {
        if (-not (Test-Path -LiteralPath $env:ADER_BUILD_ENV_OP -PathType Leaf)) { throw 'Isolated OP preparation did not succeed.' }
        & go test -tags e2e -count=1 -v -timeout 3m "-run=^TestWindowsBuildEnvironment_$Facet`$" ./ader/catalogues/grace-op/integration/windows_build_environment
        if ($LASTEXITCODE -ne 0) { throw "Build-environment $Facet proof failed: $LASTEXITCODE" }
    }
} finally { Pop-Location }
