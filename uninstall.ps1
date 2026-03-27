#Requires -Version 5.1
<#
.SYNOPSIS
    Uninstalls ticky on Windows.

.DESCRIPTION
    Removes the ticky binary and install directory from
    $env:LOCALAPPDATA\Programs\ticky, and removes that directory from your
    user PATH in the registry. No admin rights required.

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
# 1. Remove install directory
# ---------------------------------------------------------------------------
Write-Step 'Removing ticky binary...'
if (Test-Path $InstallDir) {
    Remove-Item -Path $InstallDir -Recurse -Force
    Write-Ok "Removed: $InstallDir"
} else {
    Write-Note "Install directory not found (already removed?): $InstallDir"
}

# ---------------------------------------------------------------------------
# 2. Remove install dir from user PATH
# ---------------------------------------------------------------------------
Write-Step 'Updating user PATH...'
$registryPath = 'HKCU:\Environment'
$currentPath  = (Get-ItemProperty -Path $registryPath -Name Path -ErrorAction SilentlyContinue).Path

if (-not $currentPath) {
    Write-Note 'User PATH entry not found in registry — nothing to remove.'
} else {
    $parts   = $currentPath -split ';' | Where-Object { $_ -ne $InstallDir -and $_ -ne '' }
    $newPath = $parts -join ';'

    if ($newPath -eq $currentPath) {
        Write-Note "$InstallDir was not in user PATH."
    } else {
        Set-ItemProperty -Path $registryPath -Name Path -Value $newPath
        Write-Ok "Removed $InstallDir from user PATH."

        # Broadcast WM_SETTINGCHANGE so Explorer and new terminals pick up
        # the change without requiring a logoff.
        $signature = @'
[DllImport("user32.dll", SetLastError=true, CharSet=CharSet.Auto)]
public static extern IntPtr SendMessageTimeout(
    IntPtr hWnd, uint Msg, UIntPtr wParam, string lParam,
    uint fuFlags, uint uTimeout, out UIntPtr lpdwResult);
'@
        $type   = Add-Type -MemberDefinition $signature -Name WinEnvU -Namespace Win32 -PassThru
        $result = [UIntPtr]::Zero
        $type::SendMessageTimeout(
            [IntPtr]0xffff, 0x001A, [UIntPtr]::Zero, 'Environment',
            0x0002, 5000, [ref]$result
        ) | Out-Null
    }
}

# ---------------------------------------------------------------------------
# 3. Remove shell prompt integration
# ---------------------------------------------------------------------------
Write-Step 'Removing shell prompt integration...'

# Remove tmux integration.
$TmuxCfgXdg    = Join-Path $env:USERPROFILE '.config\tmux\tmux.conf'
$TmuxCfgLegacy = Join-Path $env:USERPROFILE '.tmux.conf'
$TmuxCfgPath   = if (Test-Path $TmuxCfgXdg) { $TmuxCfgXdg } elseif (Test-Path $TmuxCfgLegacy) { $TmuxCfgLegacy } else { $null }

if ($TmuxCfgPath -and (Test-Path $TmuxCfgPath)) {
    $content = Get-Content $TmuxCfgPath -Raw -ErrorAction SilentlyContinue
    if ($content -match 'ticky shell integration') {
        $cleaned = $content -replace '(?m)\r?\n# ticky shell integration\r?\nset -g status-interval[^\n]*\r?\nset -g status-right-length[^\n]*\r?\nset -g status-right[^\n]*\r?\n', "`n"
        $cleaned = $cleaned -replace '(?m)^# ticky shell integration\r?\n', ''
        Set-Content -Path $TmuxCfgPath -Value $cleaned.TrimEnd() -NoNewline
        Write-Ok "Removed ticky integration from $TmuxCfgPath"
    } else {
        Write-Note 'tmux config: no ticky integration found.'
    }
}

# Remove from PowerShell $PROFILE if present.
$ProfilePath = $PROFILE
if (-not $ProfilePath) {
    $ProfilePath = Join-Path $env:USERPROFILE 'Documents\PowerShell\Microsoft.PowerShell_profile.ps1'
}
if (Test-Path $ProfilePath) {
    $content = Get-Content $ProfilePath -Raw -ErrorAction SilentlyContinue
    if ($content -match 'ticky shell integration') {
        $cleaned = $content -replace '(?ms)\r?\n# ticky shell integration\r?\nfunction prompt \{[^\}]*\}\r?\n', ''
        $cleaned = $cleaned -replace '(?m)^# ticky shell integration\r?\n', ''
        Set-Content -Path $ProfilePath -Value $cleaned.TrimEnd()
        Write-Ok "Removed ticky integration from $ProfilePath"
    } else {
        Write-Note 'PowerShell profile: no ticky integration found.'
    }
}

# ---------------------------------------------------------------------------
# 4. Done
# ---------------------------------------------------------------------------
$ConfigDir = Join-Path $env:APPDATA 'delbysoft'

Write-Host ''
Write-Ok 'ticky has been uninstalled.'
Write-Host ''
Write-Note 'Your config and task data have been left in place:'
Write-Note "  $ConfigDir\ticky.toml"
Write-Note "  $ConfigDir\tasks.toml"
Write-Note 'Delete that folder manually if you want a full clean uninstall.'
Write-Host ''
