@echo off
rem Installs ticky on Windows from Command Prompt (no PowerShell required).
rem
rem   install.cmd                      build (or use releases\windows) and install
rem   install.cmd --update             used by the in-app updater; skips prompt setup
rem   install.cmd --build-dir <dir>    write build output to <dir> instead of bin\
rem
rem The real work is done by `ticky --install`, shared with install.ps1.
setlocal EnableExtensions DisableDelayedExpansion

set "ROOT=%~dp0"
if "%ROOT:~-1%"=="\" set "ROOT=%ROOT:~0,-1%"
set "UPDATE_FLAG="
set "BUILD_DIR=%ROOT%\bin"

:parse
if "%~1"=="" goto parsed
if /i "%~1"=="--update" (
    set "UPDATE_FLAG=--update"
    shift
    goto parse
)
if /i "%~1"=="--build-dir" (
    if "%~2"=="" goto usage
    set "BUILD_DIR=%~2"
    shift
    shift
    goto parse
)
if /i "%~1"=="--help" goto usage
if /i "%~1"=="/?" goto usage
echo Unknown option: %~1
goto usage

:parsed
pushd "%ROOT%" || exit /b 1

set "SRC="
set "DESKTOP_SRC="
where go >nul 2>&1
if errorlevel 1 goto prebuilt

echo ==^> Go found - building ticky from source...
if not exist "%BUILD_DIR%" mkdir "%BUILD_DIR%"
set "COMMIT=dev"
for /f "usebackq delims=" %%c in (`git rev-parse HEAD 2^>nul`) do set "COMMIT=%%c"
go build -ldflags="-s -w -X github.com/wingitman/ticky/internal/version.Commit=%COMMIT%" -o "%BUILD_DIR%\ticky.exe" .
if errorlevel 1 (
    echo ERROR: go build failed.
    popd
    exit /b 1
)
echo     Built: %BUILD_DIR%\ticky.exe
set "SRC=%BUILD_DIR%\ticky.exe"
go build -tags desktop -ldflags="-s -w -X github.com/wingitman/ticky/internal/version.Commit=%COMMIT%" -o "%BUILD_DIR%\ticky-desktop.exe" .\cmd\ticky-desktop
if errorlevel 1 (
    echo     WARNING: desktop build failed; installing the terminal app only.
) else (
    set "DESKTOP_SRC=%BUILD_DIR%\ticky-desktop.exe"
)
goto install

:prebuilt
echo ==^> Go not found - using pre-built binary from releases\windows\...
set "SRC=%ROOT%\releases\windows\ticky.exe"
if not exist "%SRC%" (
    echo ERROR: Pre-built binary not found at releases\windows\ticky.exe
    echo        Install Go from https://go.dev/dl/ and re-run, or ask a developer
    echo        to run "make build-all" and commit the releases\ folder.
    popd
    exit /b 1
)
if exist "%ROOT%\releases\windows\ticky-desktop.exe" set "DESKTOP_SRC=%ROOT%\releases\windows\ticky-desktop.exe"

:install
if defined DESKTOP_SRC (
    "%SRC%" --install --from "%SRC%" --desktop-from "%DESKTOP_SRC%" %UPDATE_FLAG%
) else (
    "%SRC%" --install --from "%SRC%" %UPDATE_FLAG%
)
set "RC=%ERRORLEVEL%"
popd

if defined UPDATE_FLAG exit /b %RC%
call :pause_if_double_clicked
exit /b %RC%

:pause_if_double_clicked
rem Explorer runs scripts as `cmd /c "...install.cmd"`; keep that window open.
setlocal EnableDelayedExpansion
set "CCL=!CMDCMDLINE!"
if defined CCL set "CCL=!CCL:"=!"
if defined CCL if /i not "!CCL:/c =!"=="!CCL!" pause
endlocal
exit /b 0

:usage
echo Usage: install.cmd [--update] [--build-dir ^<dir^>]
exit /b 2
