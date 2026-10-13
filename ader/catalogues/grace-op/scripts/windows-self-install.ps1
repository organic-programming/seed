#requires -Version 7.0
[CmdletBinding()]
param()
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
if (-not $IsWindows) { throw 'This proof requires native Windows; cross-compilation is insufficient.' }
$snapshot = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '../../../..'))
if (-not $env:ADER_REPO_ROOT -or -not $env:ADER_RUN_ARTIFACTS -or
    $snapshot -ne [IO.Path]::GetFullPath($env:ADER_REPO_ROOT)) {
    throw 'Run this proof through Ader against its isolated snapshot.'
}
$evidence = Join-Path $env:ADER_RUN_ARTIFACTS 'windows-self-install'
New-Item -ItemType Directory -Path $evidence -Force | Out-Null
$env:OPPATH = Join-Path $evidence 'op-home'
$env:OPBIN = Join-Path $env:OPPATH 'bin'
$env:OPROOT = $snapshot
$record = [ordered]@{
    schema = 'op.windows-self-install-proof/v1'
    status = 'running'
    snapshot = $snapshot
    scope = 'Native tests exercise the shared Install implementation in real mapped PE subprocesses, using temporary OPBIN and PATH entries only. No operator executable, shell profile or persistent PATH is modified. This focused check does not claim full op_install CLI/API/RPC parity.'
    startedAt = (Get-Date).ToUniversalTime().ToString('o')
}
Push-Location (Join-Path $snapshot 'holons/grace-op')
try {
    & go test ./internal/holons -run 'TestWindows(SelfInstall|InstallRunningExecutable)' -count=1 -v -timeout 2m 2>&1 | Tee-Object -FilePath (Join-Path $evidence 'native-tests.log')
    $record.exitCode = $LASTEXITCODE
    if ($LASTEXITCODE -ne 0) { throw "Windows self-install proof failed: $LASTEXITCODE" }
    $record.status = 'passed'
} catch {
    $record.status = 'failed'
    $record.error = $_.Exception.Message
    throw
} finally {
    Pop-Location
    $record.finishedAt = (Get-Date).ToUniversalTime().ToString('o')
    $record | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath (Join-Path $evidence 'result.json') -Encoding utf8
}
