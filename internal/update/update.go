package update

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/wingitman/ticky/internal/config"
)

const defaultRepoURL = "https://github.com/wingitman/ticky.git"

// Commit is one git commit shown in update prompts/history.
type Commit struct {
	Hash    string
	Short   string
	Subject string
	Body    string
	Date    string
}

// Info describes the local source checkout and available upstream commits.
type Info struct {
	RepoPath       string
	Branch         string
	Upstream       string
	CurrentCommit  string
	LatestCommit   string
	Available      []Commit
	History        []Commit
	CheckError     string
	UpdatesEnabled bool
}

// InstallRequest describes the install the detached helper should run.
type InstallRequest struct {
	RepoPath       string
	TargetCommit   string
	Latest         bool
	Terminal       string
	RecorderBinary string
	// WindowsShell is the windows_shell preference: auto | powershell | cmd.
	WindowsShell string
}

// Check ensures a source checkout exists, fetches its origin, and returns the
// commits newer than currentCommit on the checkout's current branch/upstream.
func Check(cfg *config.Config, currentCommit string, historyLimit int) Info {
	info := Info{UpdatesEnabled: cfg == nil || !cfg.Updates.DisableChecks}
	if cfg != nil && cfg.Updates.DisableChecks {
		return info
	}

	repoPath, err := ensureRepoPath(cfg)
	if err != nil {
		info.CheckError = err.Error()
		return info
	}
	info.RepoPath = repoPath

	if out, err := git(repoPath, "fetch", "--prune", "--all"); err != nil {
		info.CheckError = strings.TrimSpace(out)
		if info.CheckError == "" {
			info.CheckError = err.Error()
		}
		return info
	}

	branch, _ := gitTrim(repoPath, "rev-parse", "--abbrev-ref", "HEAD")
	info.Branch = branch
	upstream, err := gitTrim(repoPath, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	if err != nil || upstream == "" {
		if branch != "" && branch != "HEAD" {
			upstream = "origin/" + branch
		} else {
			upstream = "origin/HEAD"
		}
	}
	info.Upstream = upstream

	if currentCommit == "" || currentCommit == "dev" {
		if cfg != nil && cfg.Updates.CurrentCommit != "" {
			currentCommit = cfg.Updates.CurrentCommit
		}
	}
	if currentCommit == "" || currentCommit == "dev" {
		currentCommit, _ = gitTrim(repoPath, "rev-parse", "HEAD")
	}
	info.CurrentCommit = currentCommit
	info.LatestCommit, _ = gitTrim(repoPath, "rev-parse", upstream)

	if currentCommit != "" && info.LatestCommit != "" && currentCommit != info.LatestCommit {
		info.Available = gitLog(repoPath, fmt.Sprintf("%s..%s", currentCommit, upstream), historyLimit)
	}
	info.History = gitLog(repoPath, "HEAD", historyLimit)
	return info
}

// LaunchDetached writes an update script and opens it in a separate terminal.
func LaunchDetached(req InstallRequest) error {
	if strings.TrimSpace(req.RepoPath) == "" {
		return errors.New("missing update repo path")
	}
	if runtime.GOOS == "windows" {
		return launchWindows(req)
	}
	return launchUnix(req)
}

func ensureRepoPath(cfg *config.Config) (string, error) {
	if cfg != nil && cfg.Updates.RepoPath != "" && isGitRepo(cfg.Updates.RepoPath) {
		return cfg.Updates.RepoPath, nil
	}
	if cwd, err := os.Getwd(); err == nil && isTickyRepo(cwd) {
		_ = config.RecordUpdateMetadata("", cwd)
		return cwd, nil
	}
	dir, err := config.ConfigDir()
	if err != nil {
		return "", err
	}
	repoPath := filepath.Join(dir, "ticky-src")
	if isGitRepo(repoPath) {
		_ = config.RecordUpdateMetadata("", repoPath)
		return repoPath, nil
	}
	if err := os.MkdirAll(filepath.Dir(repoPath), 0755); err != nil {
		return "", err
	}
	cmd := exec.Command("git", "clone", defaultRepoURL, repoPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("clone update repo: %v: %s", err, strings.TrimSpace(string(out)))
	}
	_ = config.RecordUpdateMetadata("", repoPath)
	return repoPath, nil
}

func isTickyRepo(path string) bool {
	if !isGitRepo(path) {
		return false
	}
	remote, err := gitTrim(path, "remote", "get-url", "origin")
	if err != nil {
		return false
	}
	return isTickyRemote(remote)
}

func isTickyRemote(remote string) bool {
	remote = strings.TrimSuffix(strings.TrimSpace(remote), ".git")
	return strings.HasSuffix(remote, "/ticky")
}

func isGitRepo(path string) bool {
	if path == "" {
		return false
	}
	cmd := exec.Command("git", "-C", path, "rev-parse", "--is-inside-work-tree")
	out, err := cmd.Output()
	return err == nil && strings.TrimSpace(string(out)) == "true"
}

func gitTrim(repoPath string, args ...string) (string, error) {
	out, err := git(repoPath, args...)
	return strings.TrimSpace(out), err
}

func git(repoPath string, args ...string) (string, error) {
	cmdArgs := append([]string{"-C", repoPath}, args...)
	cmd := exec.Command("git", cmdArgs...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func gitLog(repoPath string, rev string, limit int) []Commit {
	if limit < 1 {
		limit = 12
	}
	format := "%H%x1f%h%x1f%s%x1f%b%x1f%ad%x1e"
	args := []string{"log", "--date=short", "--format=" + format, "-n", fmt.Sprint(limit)}
	if rev != "" {
		args = append(args, rev)
	}
	out, err := git(repoPath, args...)
	if err != nil {
		return nil
	}
	records := strings.Split(out, "\x1e")
	commits := make([]Commit, 0, len(records))
	for _, rec := range records {
		rec = strings.TrimSpace(rec)
		if rec == "" {
			continue
		}
		parts := strings.SplitN(rec, "\x1f", 5)
		if len(parts) < 5 {
			continue
		}
		commits = append(commits, Commit{
			Hash:    parts[0],
			Short:   parts[1],
			Subject: parts[2],
			Body:    strings.TrimSpace(parts[3]),
			Date:    parts[4],
		})
	}
	return commits
}

func launchUnix(req InstallRequest) error {
	script, err := writeUnixScript(req)
	if err != nil {
		return err
	}
	if runtime.GOOS == "darwin" {
		cmd := exec.Command("osascript", "-e", fmt.Sprintf(`tell application "Terminal" to do script %q`, script))
		return cmd.Start()
	}
	terminal, args, err := terminalCommand(req.Terminal, script)
	if err != nil {
		return err
	}
	cmd := exec.Command(terminal, args...)
	return cmd.Start()
}

func writeUnixScript(req InstallRequest) (string, error) {
	dir, err := updatesDir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, fmt.Sprintf("update-%d.sh", time.Now().UnixNano()))
	var b bytes.Buffer
	b.WriteString("#!/bin/sh\nset -u\n")
	b.WriteString("repo=" + shQuote(req.RepoPath) + "\n")
	b.WriteString("target=" + shQuote(req.TargetCommit) + "\n")
	b.WriteString("recorder=" + shQuote(req.RecorderBinary) + "\n")
	b.WriteString("log=" + shQuote(filepath.Join(dir, fmt.Sprintf("update-%d.log", time.Now().UnixNano()))) + "\n")
	b.WriteString("work=\"\"\n")
	b.WriteString("prev_ref=HEAD\n")
	b.WriteString("status=0\n")
	b.WriteString(": >\"$log\"\n")
	b.WriteString("printf 'ticky update started\\n' | tee -a \"$log\"\n")
	b.WriteString("prev_ref=$(git -C \"$repo\" rev-parse --abbrev-ref HEAD 2>/dev/null || printf HEAD)\n")
	b.WriteString("if [ \"$prev_ref\" != HEAD ]; then upstream=$(git -C \"$repo\" rev-parse --abbrev-ref --symbolic-full-name '@{u}' 2>/dev/null || printf 'origin/%s' \"$prev_ref\"); else upstream=HEAD; fi\n")
	b.WriteString("git -C \"$repo\" fetch --prune --all >>\"$log\" 2>&1 || status=$?\n")
	if req.Latest {
		b.WriteString("if [ \"$status\" -eq 0 ] && [ \"$prev_ref\" != HEAD ]; then target=$(git -C \"$repo\" rev-parse \"$upstream\" 2>>\"$log\") || status=$?; fi\n")
		b.WriteString("if [ \"$status\" -eq 0 ] && [ \"$prev_ref\" = HEAD ] && [ -z \"$target\" ]; then target=$(git -C \"$repo\" rev-parse HEAD 2>>\"$log\") || status=$?; fi\n")
		b.WriteString("if [ \"$status\" -eq 0 ] && [ -z \"$target\" ]; then status=1; printf 'Could not resolve the latest update target.\\n' >>\"$log\"; fi\n")
	} else {
		b.WriteString("if [ \"$status\" -eq 0 ] && [ -z \"$target\" ]; then status=1; printf 'No update target was provided.\\n' >>\"$log\"; fi\n")
	}
	b.WriteString("if [ \"$status\" -eq 0 ]; then work=$(mktemp -d \"${TMPDIR:-/tmp}/ticky-update.XXXXXX\") || status=$?; fi\n")
	b.WriteString("if [ \"$status\" -eq 0 ]; then rmdir \"$work\"; git -C \"$repo\" worktree add --detach \"$work\" \"$target\" >>\"$log\" 2>&1 || status=$?; fi\n")
	b.WriteString("if [ \"$status\" -eq 0 ]; then (cd \"$work\" && make install UPDATE=1 BUILD_DIR=\"$work/build\") >>\"$log\" 2>&1 || status=$?; fi\n")
	b.WriteString("installed=$(git -C \"${work:-$repo}\" rev-parse HEAD 2>/dev/null || printf unknown)\n")
	b.WriteString("if [ \"$status\" -eq 0 ] && [ -n \"$recorder\" ] && [ -x \"$recorder\" ]; then \"$recorder\" --record-update --update-commit \"$installed\" --update-repo \"$repo\" >>\"$log\" 2>&1 || status=$?; fi\n")
	b.WriteString("if [ -n \"$work\" ]; then git -C \"$repo\" worktree remove --force \"$work\" >>\"$log\" 2>&1 || status=$?; fi\n")
	b.WriteString("printf '\\n--- updater output ---\\n'; cat \"$log\"\n")
	b.WriteString("if [ \"$status\" -eq 0 ]; then printf '\\nticky update complete: %s\\n' \"$installed\"; else printf '\\nticky update failed (exit %s). Log: %s\\n' \"$status\" \"$log\"; fi\n")
	b.WriteString("printf 'Press Enter to close...'; read _ || true\n")
	b.WriteString("exit \"$status\"\n")
	if err := os.WriteFile(path, b.Bytes(), 0755); err != nil {
		return "", err
	}
	return path, nil
}

// Windows update shells.
const (
	shellPowerShell = "powershell"
	shellCmd        = "cmd"
)

// chooseWindowsShell resolves the windows_shell preference. "auto" uses
// PowerShell only when the probe confirms it can run an unsigned script in
// FullLanguage mode; machines where a domain policy blocks or constrains
// PowerShell fall back to Command Prompt.
func chooseWindowsShell(pref string, powerShellUsable func() bool) string {
	switch config.NormalizeWindowsShell(pref) {
	case shellPowerShell:
		return shellPowerShell
	case shellCmd:
		return shellCmd
	}
	if powerShellUsable != nil && powerShellUsable() {
		return shellPowerShell
	}
	return shellCmd
}

// powerShellProbeScript prints the language mode. Constrained Language Mode
// (AppLocker/WDAC) breaks the updater's .NET calls, so only FullLanguage
// counts as usable.
const powerShellProbeScript = "$ExecutionContext.SessionState.LanguageMode\r\n"

func updatesDir() (string, error) {
	dir := filepath.Join(os.TempDir(), "ticky-updates")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

// writeWindowsScript writes the PowerShell updater. It builds the target
// commit in a temporary worktree and installs it with the currently running
// ticky's `--install`, so rolling back to commits that predate --install
// still works.
func writeWindowsScript(req InstallRequest) (string, error) {
	dir, err := updatesDir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, fmt.Sprintf("update-%d.ps1", time.Now().UnixNano()))
	latest := "$false"
	if req.Latest {
		latest = "$true"
	}
	content := fmt.Sprintf(`$ErrorActionPreference = 'Continue'
$repo = %s
$target = %s
$recorder = %s
$latest = %s
$status = 0
$log = Join-Path ([System.IO.Path]::GetTempPath()) ('ticky-update-' + [guid]::NewGuid().ToString() + '.log')
$work = $null
$commit = 'unknown'
$prevRef = (git -C $repo rev-parse --abbrev-ref HEAD 2>$null)
if ($prevRef) { $prevRef = $prevRef.Trim() } else { $prevRef = 'HEAD' }
"ticky update started" | Tee-Object -FilePath $log
git -C $repo fetch --prune --all *>> $log
if ($LASTEXITCODE -ne 0) { $status = $LASTEXITCODE }
if ($status -eq 0) {
    if ($latest -and $prevRef -ne 'HEAD') {
        $upstream = (git -C $repo rev-parse --abbrev-ref --symbolic-full-name '@{u}' 2>$null)
        if ($upstream) { $upstream = $upstream.Trim() } else { $upstream = "origin/$prevRef" }
        $target = (git -C $repo rev-parse $upstream 2>> $log)
        if ($target) { $target = $target.Trim() }
    }
    if (-not $target) { $status = 1; 'Could not resolve the update target.' | Add-Content $log }
}
if ($status -eq 0) {
    $work = Join-Path ([System.IO.Path]::GetTempPath()) ('ticky-update-' + [guid]::NewGuid().ToString())
    git -C $repo worktree add --detach $work $target *>> $log
    if ($LASTEXITCODE -ne 0) { $status = $LASTEXITCODE; $work = $null }
}
$built = $null
$desktop = $null
if ($status -eq 0) {
    $commit = (git -C $work rev-parse HEAD).Trim()
    $built = Join-Path $work 'build\ticky.exe'
    $desktop = Join-Path $work 'build\ticky-desktop.exe'
    Push-Location $work
    go build -ldflags="-s -w -X github.com/wingitman/ticky/internal/version.Commit=$commit" -o $built . *>> $log
    if ($LASTEXITCODE -ne 0) { $status = $LASTEXITCODE }
    if ($status -eq 0) {
        go build -tags desktop -ldflags="-s -w -X github.com/wingitman/ticky/internal/version.Commit=$commit" -o $desktop ./cmd/ticky-desktop *>> $log
        if ($LASTEXITCODE -ne 0) { $desktop = $null; 'Desktop build failed; continuing without it.' | Add-Content $log }
    }
    Pop-Location
}
if ($status -eq 0) {
    $installer = if ($recorder -and (Test-Path $recorder)) { $recorder } else { $built }
    $installArgs = @('--install', '--update', '--from', $built)
    if ($desktop) { $installArgs += @('--desktop-from', $desktop) }
    & $installer @installArgs *>> $log
    if ($LASTEXITCODE -ne 0) { $status = $LASTEXITCODE }
    if ($status -eq 0) {
        & $installer --record-update --update-commit $commit --update-repo $repo *>> $log
        if ($LASTEXITCODE -ne 0) { $status = $LASTEXITCODE }
    }
}
if ($work) { git -C $repo worktree remove --force $work *>> $log; if ($LASTEXITCODE -ne 0) { $status = 1 } }
Write-Host ""
Write-Host "--- updater output ---"
Get-Content $log
if ($status -eq 0) { Write-Host "ticky update complete: $commit" -ForegroundColor Green } else { Write-Host "ticky update failed (exit $status). Log: $log" -ForegroundColor Red }
Read-Host 'Press Enter to close'
exit $status
`, psQuote(req.RepoPath), psQuote(req.TargetCommit), psQuote(req.RecorderBinary), latest)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", err
	}
	return path, nil
}

// writeCmdScript writes the Command Prompt updater, used when PowerShell is
// blocked. It mirrors writeWindowsScript. Control flow uses goto rather than
// parenthesised blocks so %%VAR%% values are expanded after they are set,
// and delayed expansion stays off so "!" in paths is literal.
func writeCmdScript(req InstallRequest) (string, error) {
	dir, err := updatesDir()
	if err != nil {
		return "", err
	}
	stamp := time.Now().UnixNano()
	path := filepath.Join(dir, fmt.Sprintf("update-%d.cmd", stamp))
	logPath := filepath.Join(dir, fmt.Sprintf("update-%d.log", stamp))
	work := filepath.Join(os.TempDir(), fmt.Sprintf("ticky-update-%d", stamp))
	latest := "0"
	if req.Latest {
		latest = "1"
	}
	const ldflags = `-ldflags="-s -w -X github.com/wingitman/ticky/internal/version.Commit=%COMMIT%"`
	lines := []string{
		"@echo off",
		"setlocal EnableExtensions DisableDelayedExpansion",
		"title ticky update",
		cmdSet("REPO", req.RepoPath),
		cmdSet("TARGET", req.TargetCommit),
		cmdSet("RECORDER", req.RecorderBinary),
		cmdSet("LATEST", latest),
		cmdSet("LOG", logPath),
		cmdSet("WORK", work),
		`set "STATUS=0"`,
		`set "WORKTREE="`,
		`set "COMMIT=unknown"`,
		`type nul > "%LOG%"`,
		"echo ticky update started",
		`echo ticky update started>>"%LOG%"`,
		`set "PREV_REF=HEAD"`,
		cmdCapture("r", "git -C \"%REPO%\" rev-parse --abbrev-ref HEAD 2^>nul", "PREV_REF"),
		`git -C "%REPO%" fetch --prune --all >>"%LOG%" 2>&1`,
		`if errorlevel 1 set "STATUS=%ERRORLEVEL%"`,
		`if not "%STATUS%"=="0" goto finish`,
		`if "%LATEST%"=="1" if not "%PREV_REF%"=="HEAD" call :resolve_latest`,
		`if not defined TARGET echo Could not resolve the update target.>>"%LOG%"`,
		`if not defined TARGET set "STATUS=1"`,
		`if not "%STATUS%"=="0" goto finish`,
		`git -C "%REPO%" worktree add --detach "%WORK%" "%TARGET%" >>"%LOG%" 2>&1`,
		`if errorlevel 1 set "STATUS=%ERRORLEVEL%"`,
		`if not "%STATUS%"=="0" goto finish`,
		`set "WORKTREE=1"`,
		cmdCapture("c", "git -C \"%WORK%\" rev-parse HEAD 2^>nul", "COMMIT"),
		`set "BUILT=%WORK%\build\ticky.exe"`,
		`set "DESKTOP=%WORK%\build\ticky-desktop.exe"`,
		`pushd "%WORK%"`,
		`go build ` + ldflags + ` -o "%BUILT%" . >>"%LOG%" 2>&1`,
		`if errorlevel 1 set "STATUS=%ERRORLEVEL%"`,
		`if not "%STATUS%"=="0" popd`,
		`if not "%STATUS%"=="0" goto finish`,
		`go build -tags desktop ` + ldflags + ` -o "%DESKTOP%" .\cmd\ticky-desktop >>"%LOG%" 2>&1`,
		`if errorlevel 1 set "DESKTOP="`,
		`popd`,
		`if not defined DESKTOP echo Desktop build failed; continuing without it.>>"%LOG%"`,
		`set "INSTALLER=%BUILT%"`,
		`if defined RECORDER if exist "%RECORDER%" set "INSTALLER=%RECORDER%"`,
		`if defined DESKTOP goto install_with_desktop`,
		`"%INSTALLER%" --install --update --from "%BUILT%" >>"%LOG%" 2>&1`,
		`goto installed`,
		`:install_with_desktop`,
		`"%INSTALLER%" --install --update --from "%BUILT%" --desktop-from "%DESKTOP%" >>"%LOG%" 2>&1`,
		`:installed`,
		`if errorlevel 1 set "STATUS=%ERRORLEVEL%"`,
		`if not "%STATUS%"=="0" goto finish`,
		`"%INSTALLER%" --record-update --update-commit "%COMMIT%" --update-repo "%REPO%" >>"%LOG%" 2>&1`,
		`if errorlevel 1 set "STATUS=%ERRORLEVEL%"`,
		`:finish`,
		`if defined WORKTREE git -C "%REPO%" worktree remove --force "%WORK%" >>"%LOG%" 2>&1`,
		`if defined WORKTREE if errorlevel 1 set "STATUS=1"`,
		"echo.",
		"echo --- updater output ---",
		`type "%LOG%"`,
		"echo.",
		`if not "%STATUS%"=="0" goto failed`,
		"echo ticky update complete: %COMMIT%",
		"goto done",
		":failed",
		`echo ticky update failed (exit %STATUS%). Log: "%LOG%"`,
		":done",
		"echo Press any key to close...",
		"pause >nul",
		`exit /b %STATUS%`,
		"",
		":resolve_latest",
		`set "UPSTREAM=origin/%PREV_REF%"`,
		cmdCapture("u", "git -C \"%REPO%\" rev-parse --abbrev-ref --symbolic-full-name @{u} 2^>nul", "UPSTREAM"),
		`set "TARGET="`,
		cmdCapture("t", "git -C \"%REPO%\" rev-parse \"%UPSTREAM%\" 2^>nul", "TARGET"),
		"exit /b 0",
		"",
	}
	content := strings.Join(lines, "\r\n")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", err
	}
	return path, nil
}

// cmdCapture renders a batch line that stores the last output line of
// command in variable name. Redirections inside command must be ^-escaped.
func cmdCapture(letter, command, name string) string {
	return `for /f "usebackq delims=" %%` + letter + " in (`" + command + "`) do set \"" + name + "=%%" + letter + `"`
}

// cmdSet renders `set "NAME=value"` for a batch file with delayed expansion
// disabled. Inside the quoted form &, |, <, >, ^ and ! are literal; only %
// needs doubling. Quotes and line breaks cannot occur in Windows paths or
// commit hashes and would break the line, so they are dropped.
func cmdSet(name, value string) string {
	value = strings.NewReplacer(`"`, "", "\r", "", "\n", "", "%", "%%").Replace(value)
	return `set "` + name + "=" + value + `"`
}

func terminalCommand(preferred string, script string) (string, []string, error) {
	if preferred != "" {
		name := filepath.Base(preferred)
		if name == "konsole" || name == "konsole.exe" {
			return preferred, []string{"--hold", "-e", script}, nil
		}
		return preferred, []string{"-e", script}, nil
	}
	candidates := []struct {
		name string
		args []string
	}{
		{"x-terminal-emulator", []string{"-e", script}},
		{"gnome-terminal", []string{"--", script}},
		{"konsole", []string{"--hold", "-e", script}},
		{"xfce4-terminal", []string{"--hold", "-e", script}},
		{"alacritty", []string{"--hold", "-e", script}},
		{"kitty", []string{"--hold", script}},
		{"wezterm", []string{"start", "--", script}},
		{"foot", []string{"-e", script}},
	}
	for _, c := range candidates {
		if _, err := exec.LookPath(c.name); err == nil {
			return c.name, c.args, nil
		}
	}
	return "", nil, errors.New("no supported terminal found for detached update")
}

func shQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func psQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
