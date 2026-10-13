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
$state = Join-Path $env:ADER_RUN_ARTIFACTS 'live-artifact'
New-Item -ItemType Directory -Path $state -Force | Out-Null
$bootstrap = (Get-Command op -CommandType Application -ErrorAction Stop | Select-Object -First 1).Source
$env:OPPATH = Join-Path $state 'op-home'
$env:OPBIN = Join-Path $env:OPPATH 'bin'
$env:OPROOT = $snapshot
$env:GRACE_OP_SHARED_CACHE_DIR = Join-Path $state 'package-cache'
$env:ADER_LIVE_ARTIFACT_RESULTS = $state
$env:ADER_LIVE_ARTIFACT_OP = Join-Path $snapshot 'holons/grace-op/.op/build/grace-op.holon/bin/windows_amd64/op.exe'
Push-Location $snapshot
try {
    if ($Facet -eq 'prepare') {
        & $bootstrap --root $snapshot --no-cache build op
        if ($LASTEXITCODE -ne 0) { throw "Isolated OP build failed: $LASTEXITCODE" }
        if (-not (Test-Path -LiteralPath $env:ADER_LIVE_ARTIFACT_OP -PathType Leaf)) { throw 'Isolated OP artifact is missing.' }
        $sourceHashes = [ordered]@{}
        foreach ($source in Get-ChildItem -LiteralPath (Join-Path $snapshot 'holons/grace-op/internal/holons') -Filter 'active_artifacts_*.go') {
            $sourceHashes[$source.Name] = (Get-FileHash -LiteralPath $source.FullName).Hash
        }
        $sourceHashes['lifecycle.go'] = (Get-FileHash -LiteralPath (Join-Path $snapshot 'holons/grace-op/internal/holons/lifecycle.go')).Hash
        $provenance = [ordered]@{
            snapshot = $snapshot
            binary = $env:ADER_LIVE_ARTIFACT_OP
            sha256 = (Get-FileHash -LiteralPath $env:ADER_LIVE_ARTIFACT_OP).Hash
            bootstrapSHA256 = (Get-FileHash -LiteralPath $bootstrap).Hash
            fixtureSHA256 = (Get-FileHash -LiteralPath (Join-Path $snapshot 'ader/catalogues/grace-op/integration/windows_live_artifact/artifact_windows_test.go')).Hash
            sourceSHA256 = $sourceHashes
        } | ConvertTo-Json
        $provenance | Set-Content -LiteralPath (Join-Path $state 'op-provenance.json') -Encoding utf8
        Write-Output $provenance
    } else {
        if (-not (Test-Path -LiteralPath $env:ADER_LIVE_ARTIFACT_OP -PathType Leaf)) { throw 'Isolated OP preparation did not succeed.' }
        & go test -tags e2e -count=1 -v -timeout 3m "-run=^TestWindowsLiveArtifact_$Facet`$" ./ader/catalogues/grace-op/integration/windows_live_artifact
        if ($LASTEXITCODE -ne 0) { throw "Live-artifact $Facet proof failed: $LASTEXITCODE" }
    }
} finally { Pop-Location }
