#Requires -Version 5.1
<#
.SYNOPSIS
    Installs ticky on Windows.

.DESCRIPTION
    Builds the ticky binary with 'go build', installs it to
    $env:LOCALAPPDATA\Programs\ticky, and adds that directory to your
    user PATH (persisted via the registry) without requiring admin rights.

.EXAMPLE
    .\install.ps1
#>

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$BinaryName  = 'ticky.exe'
$InstallDir  = Join-Path $env:LOCALAPPDATA 'Programs\ticky'
$BuildDir    = Join-Path $PSScriptRoot 'bin'
$BinaryBuild = Join-Path $BuildDir $BinaryName
$BinaryDest  = Join-Path $InstallDir $BinaryName

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
# 1. Check Go is available
# ---------------------------------------------------------------------------
Write-Step 'Checking for Go...'
if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    Write-Host ''
    Write-Host 'ERROR: Go is not installed or not on PATH.' -ForegroundColor Red
    Write-Host 'Download Go from https://go.dev/dl/ and re-run this script.' -ForegroundColor Red
    exit 1
}
$goVersion = go version
Write-Ok $goVersion

# ---------------------------------------------------------------------------
# 2. Build
# ---------------------------------------------------------------------------
Write-Step 'Building ticky...'
if (-not (Test-Path $BuildDir)) {
    New-Item -ItemType Directory -Path $BuildDir | Out-Null
}
& go build -ldflags='-s -w' -o $BinaryBuild .
if ($LASTEXITCODE -ne 0) {
    Write-Host 'ERROR: go build failed.' -ForegroundColor Red
    exit 1
}
Write-Ok "Built: $BinaryBuild"

# ---------------------------------------------------------------------------
# 3. Install binary
# ---------------------------------------------------------------------------
Write-Step 'Installing binary...'
if (-not (Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir | Out-Null
}
Copy-Item -Path $BinaryBuild -Destination $BinaryDest -Force
Write-Ok "Installed: $BinaryDest"

# ---------------------------------------------------------------------------
# 4. Add install dir to user PATH (persistent, no admin required)
# ---------------------------------------------------------------------------
Write-Step 'Updating user PATH...'
$registryPath = 'HKCU:\Environment'
$currentPath  = (Get-ItemProperty -Path $registryPath -Name Path -ErrorAction SilentlyContinue).Path

if ($currentPath -and ($currentPath -split ';') -contains $InstallDir) {
    Write-Note "$InstallDir is already in your PATH."
} else {
    $newPath = if ($currentPath) { "$currentPath;$InstallDir" } else { $InstallDir }
    Set-ItemProperty -Path $registryPath -Name Path -Value $newPath
    Write-Ok "Added $InstallDir to user PATH."

    # Broadcast WM_SETTINGCHANGE so Explorer and new terminals pick up the
    # change without requiring a logoff.
    $signature = @'
[DllImport("user32.dll", SetLastError=true, CharSet=CharSet.Auto)]
public static extern IntPtr SendMessageTimeout(
    IntPtr hWnd, uint Msg, UIntPtr wParam, string lParam,
    uint fuFlags, uint uTimeout, out UIntPtr lpdwResult);
'@
    $type   = Add-Type -MemberDefinition $signature -Name WinEnv -Namespace Win32 -PassThru
    $result = [UIntPtr]::Zero
    $type::SendMessageTimeout(
        [IntPtr]0xffff, 0x001A, [UIntPtr]::Zero, 'Environment',
        0x0002, 5000, [ref]$result
    ) | Out-Null
}

# Also update the current session so the user can run ticky right away.
if (($env:PATH -split ';') -notcontains $InstallDir) {
    $env:PATH = "$env:PATH;$InstallDir"
}

# ---------------------------------------------------------------------------
# 5. Shell prompt integration
# ---------------------------------------------------------------------------
Write-Step 'Setting up shell prompt integration...'

# ── tmux (live updates, additive — runs alongside shell prompt method) ──────
$TmuxCfgXdg    = Join-Path $env:USERPROFILE '.config\tmux\tmux.conf'
$TmuxCfgLegacy = Join-Path $env:USERPROFILE '.tmux.conf'
$TmuxCfgPath   = if (Test-Path $TmuxCfgXdg) { $TmuxCfgXdg } elseif (Test-Path $TmuxCfgLegacy) { $TmuxCfgLegacy } else { $null }

if ($TmuxCfgPath) {
    $tmuxContent = Get-Content $TmuxCfgPath -Raw -ErrorAction SilentlyContinue
    if ($tmuxContent -match 'ticky shell integration') {
        Write-Note 'tmux: ticky already integrated — skipping.'
    } else {
        $tmuxBlock = "`n# ticky shell integration`nset -g status-interval 1`nset -g status-right-length 120`nset -g status-right `"#(ticky --status 2>`$null)  #[fg=blue]#{?client_prefix,PREFIX ,}#[fg=brightblack]#h `"`n"
        Add-Content -Path $TmuxCfgPath -Value $tmuxBlock
        Write-Ok "tmux: added live status bar to $TmuxCfgPath"
        Write-Note "tmux: run 'tmux source-file $TmuxCfgPath' to apply without restarting tmux"
    }
}

# ── Shell prompt ($PROFILE) — only when no tmux config was found ─────────────
if ($TmuxCfgPath) {
    Write-Note 'tmux detected — skipping PowerShell prompt integration (tmux provides live updates).'
}

if (-not $TmuxCfgPath) {

$ProfilePath = $PROFILE
if (-not $ProfilePath) {
    $ProfilePath = Join-Path $env:USERPROFILE 'Documents\PowerShell\Microsoft.PowerShell_profile.ps1'
}

$IntegrationBlock = @"

# ticky shell integration
function prompt {
    `$s = ticky --status 2>`$null
    if (`$s) { Write-Host "`$s " -NoNewline -ForegroundColor Blue }
    "PS `$(Get-Location)> "
}
"@

if (Test-Path $ProfilePath) {
    $existing = Get-Content $ProfilePath -Raw -ErrorAction SilentlyContinue
    if ($existing -match 'ticky shell integration') {
        Write-Note 'PowerShell profile: ticky already integrated — skipping.'
    } else {
        Add-Content -Path $ProfilePath -Value $IntegrationBlock
        Write-Ok "PowerShell: added ticky prompt to $ProfilePath"
    }
} else {
    $dir = Split-Path $ProfilePath
    if (-not (Test-Path $dir)) { New-Item -ItemType Directory -Path $dir -Force | Out-Null }
    Set-Content -Path $ProfilePath -Value $IntegrationBlock.TrimStart()
    Write-Ok "PowerShell: created $ProfilePath with ticky prompt"
}

} # end if (-not $TmuxCfgPath)

# ---------------------------------------------------------------------------
# 6. Done
# ---------------------------------------------------------------------------
$ConfigFile = Join-Path $env:APPDATA 'delbysoft\ticky.toml'
$DataFile   = Join-Path $env:APPDATA 'delbysoft\tasks.toml'

Write-Host ''
Write-Host '  ticky installed successfully!' -ForegroundColor Green
Write-Host ''
Write-Host '  To show the active task in your prompt, open ticky and press o,' -ForegroundColor White
Write-Host '  then enable these options in ticky.toml:' -ForegroundColor White
Write-Host '    show_task_name = true' -ForegroundColor Cyan
Write-Host '    show_time_left = true' -ForegroundColor Cyan
Write-Host ''
Write-Host '  Open a new terminal and run:' -ForegroundColor White
Write-Host '    ticky' -ForegroundColor Cyan
Write-Host ''
Write-Host '  Config file (created on first launch):' -ForegroundColor White
Write-Host "    $ConfigFile" -ForegroundColor Cyan
Write-Host ''
Write-Host '  Task data file (created on first task):' -ForegroundColor White
Write-Host "    $DataFile" -ForegroundColor Cyan
Write-Host ''
Write-Note "  Tip: if you get an 'execution policy' error, run once as your user:"
Write-Note '    Set-ExecutionPolicy -Scope CurrentUser RemoteSigned'
Write-Host ''
