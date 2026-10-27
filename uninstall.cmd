@echo off
rem Uninstalls ticky on Windows from Command Prompt (no PowerShell required).
rem Removes the install folder, the user PATH entry, and prompt integration.
rem Config and task data in %APPDATA%\delbysoft are left in place.
setlocal EnableExtensions DisableDelayedExpansion

set "TICKY=%LOCALAPPDATA%\Programs\ticky\ticky.exe"
if not exist "%TICKY%" (
    echo ticky is not installed at %TICKY%
    exit /b 1
)
"%TICKY%" --uninstall
set "RC=%ERRORLEVEL%"

rem Explorer runs scripts as `cmd /c "...uninstall.cmd"`; keep that window open.
setlocal EnableDelayedExpansion
set "CCL=!CMDCMDLINE!"
if defined CCL set "CCL=!CCL:"=!"
if defined CCL if /i not "!CCL:/c =!"=="!CCL!" pause
endlocal
exit /b %RC%
