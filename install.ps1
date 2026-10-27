#Requires -Version 5.1
<#
.SYNOPSIS
    Installs ticky on Windows.

.DESCRIPTION
    Builds the ticky binary with 'go build' (or uses releases\windows), then
    runs `ticky --install`, which copies it to $env:LOCALAPPDATA\Programs\ticky,
    adds that directory to your user PATH without requiring admin rights, and
    adds the prompt integration to your PowerShell profile.

    install.cmd does the same from Command Prompt when PowerShell is blocked.

.EXAMPLE
    .\install.ps1
#>

param(
    [switch]$Update,
    [string]$BuildDirOverride = '',
    [Alias('build-all', '--build-all')]
    [switch]$BuildAll
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$BinaryName  = 'ticky.exe'
$BuildDir    = if ($BuildDirOverride) { $BuildDirOverride } else { Join-Path $PSScriptRoot 'bin' }
$BinaryBuild = Join-Path $BuildDir $BinaryName
$DesktopBinaryName = 'ticky-desktop.exe'
$DesktopBuild = Join-Path $BuildDir $DesktopBinaryName
$ReleaseBin  = Join-Path $PSScriptRoot 'releases\windows\ticky.exe'
$ReleaseDesktop = Join-Path $PSScriptRoot 'releases\windows\ticky-desktop.exe'

function Write-Step([string]$msg) {
    Write-Host "==> $msg" -ForegroundColor Cyan
}

function Write-Ok([string]$msg) {
    Write-Host "    $msg" -ForegroundColor Green
}

function Write-Note([string]$msg) {
    Write-Host "    $msg" -ForegroundColor Yellow
}

# ---------------------------------------------------------------------------
# 1. Build release binaries, or obtain the install binary
# ---------------------------------------------------------------------------
if ($BuildAll) {
    Write-Step 'Checking for Go (required for -BuildAll)...'
    if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
        Write-Host 'ERROR: Go is not installed or not on PATH.' -ForegroundColor Red
        Write-Host 'Download Go from https://go.dev/dl/ and re-run with -BuildAll.' -ForegroundColor Red
        exit 1
    }
    Write-Ok (go version)
    $Commit = (git rev-parse HEAD 2>$null)
    if (-not $Commit) { $Commit = 'dev' }
    $Targets = @(
        @{ GOOS = 'linux';   GOARCH = 'amd64'; Out = 'releases\linux\amd64\ticky' },
        @{ GOOS = 'linux';   GOARCH = 'arm64'; Out = 'releases\linux\arm64\ticky' },
        @{ GOOS = 'darwin';  GOARCH = 'amd64'; Out = 'releases\darwin\amd64\ticky' },
        @{ GOOS = 'darwin';  GOARCH = 'arm64'; Out = 'releases\darwin\arm64\ticky' },
        @{ GOOS = 'windows'; GOARCH = 'amd64'; Out = 'releases\windows\ticky.exe' }
    )
    foreach ($t in $Targets) {
        Write-Step "Building $($t.GOOS)/$($t.GOARCH)..."
        $out = Join-Path $PSScriptRoot $t.Out
        New-Item -ItemType Directory -Path (Split-Path $out) -Force | Out-Null
        $env:GOOS = $t.GOOS
        $env:GOARCH = $t.GOARCH
        $env:CGO_ENABLED = '0'
        & go build -ldflags="-s -w -X github.com/wingitman/ticky/internal/version.Commit=$Commit" -o $out .
        if ($LASTEXITCODE -ne 0) { Write-Host "ERROR: build failed for $($t.GOOS)/$($t.GOARCH)." -ForegroundColor Red; exit 1 }
        Write-Ok $t.Out
    }
    $env:GOOS = $null
    $env:GOARCH = $null
    $env:CGO_ENABLED = $null
    Write-Host ''
    Write-Host '  Pre-built binaries written to releases\' -ForegroundColor Green
    exit 0
}

# Detached update scripts call this installer after checking out the target
# commit. Do not modify shell profiles or print first-install instructions.
$DesktopSource = $null

if (Get-Command go -ErrorAction SilentlyContinue) {
    Write-Step 'Go found - building ticky from source...'
    Write-Ok (go version)
    if (-not (Test-Path $BuildDir)) { New-Item -ItemType Directory -Path $BuildDir | Out-Null }
    $Commit = (git rev-parse HEAD 2>$null)
    if (-not $Commit) { $Commit = 'dev' }
    & go build -ldflags="-s -w -X github.com/wingitman/ticky/internal/version.Commit=$Commit" -o $BinaryBuild .
    if ($LASTEXITCODE -ne 0) { Write-Host 'ERROR: go build failed.' -ForegroundColor Red; exit 1 }
    Write-Ok "Built: $BinaryBuild"
    & go build -tags desktop -ldflags="-s -w -X github.com/wingitman/ticky/internal/version.Commit=$Commit" -o $DesktopBuild ./cmd/ticky-desktop
    if ($LASTEXITCODE -ne 0) {
        Write-Note 'WARNING: desktop build failed; installing the terminal app only.'
    } else {
        $DesktopSource = $DesktopBuild
    }
    $SourceBin = $BinaryBuild
} else {
    Write-Step 'Go not found - using pre-built binary from releases\windows\...'
    if (-not (Test-Path $ReleaseBin)) {
        Write-Host 'ERROR: Pre-built binary not found at releases\windows\ticky.exe' -ForegroundColor Red
        Write-Host '       Please install Go (https://go.dev/dl/) and re-run, or ask a developer to run ".\install.ps1 -BuildAll" and commit the releases\ folder.' -ForegroundColor Red
        exit 1
    }
    Write-Ok "Using: $ReleaseBin"
    $SourceBin = $ReleaseBin
    if (Test-Path $ReleaseDesktop) { $DesktopSource = $ReleaseDesktop }
}

# ---------------------------------------------------------------------------
# 2. Install. The copy, PATH update, and prompt integration are implemented in
#    `ticky --install` so install.cmd (Command Prompt) behaves identically.
# ---------------------------------------------------------------------------
$installArgs = @('--install', '--from', $SourceBin)
if ($DesktopSource) { $installArgs += @('--desktop-from', $DesktopSource) }
if ($Update) {
    $installArgs += '--update'
} elseif ($PROFILE) {
    $installArgs += @('--powershell-profile', $PROFILE)
}
& $SourceBin @installArgs
if ($LASTEXITCODE -ne 0) { Write-Host 'ERROR: ticky --install failed.' -ForegroundColor Red; exit $LASTEXITCODE }

if (-not $Update) {
    Write-Host ''
    Write-Note "Tip: if you get an 'execution policy' error, run once as your user:"
    Write-Note '  Set-ExecutionPolicy -Scope CurrentUser RemoteSigned'
    Write-Note 'If PowerShell is blocked on this machine, use install.cmd instead.'
}
