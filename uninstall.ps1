#Requires -Version 5.1
<#
.SYNOPSIS
    Uninstalls ticky on Windows.

.DESCRIPTION
    Runs `ticky --uninstall`, which removes the install directory
    $env:LOCALAPPDATA\Programs\ticky, its user PATH entry, and the prompt
    integration. No admin rights required. uninstall.cmd does the same from
    Command Prompt.

    Your config and task data files are left untouched. Remove them manually
    if you want a full clean uninstall:
        $env:APPDATA\delbysoft\ticky.toml
        $env:APPDATA\delbysoft\tasks.toml

.EXAMPLE
    .\uninstall.ps1
#>

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$InstallDir = Join-Path $env:LOCALAPPDATA 'Programs\ticky'

function Write-Note([string]$msg) {
    Write-Host "    $msg" -ForegroundColor Yellow
}

# The PATH, prompt-integration, and file removal are implemented in
# `ticky --uninstall` so uninstall.cmd (Command Prompt) behaves identically.
$Ticky = Join-Path $InstallDir 'ticky.exe'
if (-not (Test-Path $Ticky)) {
    Write-Note "ticky is not installed at $Ticky"
    exit 1
}
& $Ticky --uninstall
exit $LASTEXITCODE
