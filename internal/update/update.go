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
	dir := filepath.Join(os.TempDir(), "ticky-updates")
	if err := os.MkdirAll(dir, 0755); err != nil {
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

func launchWindows(req InstallRequest) error {
	script, err := writeWindowsScript(req)
	if err != nil {
		return err
	}
	if req.Terminal != "" {
		return exec.Command(req.Terminal, script).Start()
	}
	if _, err := exec.LookPath("wt.exe"); err == nil {
		return exec.Command("wt.exe", "powershell.exe", "-NoExit", "-ExecutionPolicy", "Bypass", "-File", script).Start()
	}
	return exec.Command("powershell.exe", "-NoExit", "-ExecutionPolicy", "Bypass", "-File", script).Start()
}

func writeWindowsScript(req InstallRequest) (string, error) {
	dir := filepath.Join(os.TempDir(), "ticky-updates")
	if err := os.MkdirAll(dir, 0755); err != nil {
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
$prevRef = (git -C $repo rev-parse --abbrev-ref HEAD 2>$null).Trim()
if (-not $prevRef) { $prevRef = 'HEAD' }
"ticky update started" | Tee-Object -FilePath $log
git -C $repo fetch --prune --all *>> $log
if ($LASTEXITCODE -ne 0) { $status = $LASTEXITCODE }
if ($status -eq 0) {
    if ($latest -and $prevRef -ne 'HEAD') {
        $upstream = (git -C $repo rev-parse --abbrev-ref --symbolic-full-name '@{u}' 2>$null).Trim()
        if (-not $upstream) { $upstream = "origin/$prevRef" }
        $target = (git -C $repo rev-parse $upstream 2>> $log).Trim()
    }
    if (-not $target) { $status = 1; 'Could not resolve the update target.' | Add-Content $log }
}
if ($status -eq 0) {
    $work = Join-Path ([System.IO.Path]::GetTempPath()) ('ticky-update-' + [guid]::NewGuid().ToString())
    git -C $repo worktree add --detach $work $target *>> $log
    if ($LASTEXITCODE -ne 0) { $status = $LASTEXITCODE }
}
if ($status -eq 0) {
    Push-Location $work
    & .\install.ps1 -Update -BuildDirOverride (Join-Path $work 'build') *>> $log
    if ($LASTEXITCODE -ne 0) { $status = $LASTEXITCODE }
    Pop-Location
}
$installed = if ($work) { (git -C $work rev-parse HEAD).Trim() } else { 'unknown' }
if ($status -eq 0 -and $recorder -and (Test-Path $recorder)) { & $recorder --record-update --update-commit $installed --update-repo $repo *>> $log; if ($LASTEXITCODE -ne 0) { $status = $LASTEXITCODE } }
if ($work) { git -C $repo worktree remove --force $work *>> $log; if ($LASTEXITCODE -ne 0) { $status = 1 } }
Write-Host ""
Write-Host "--- updater output ---"
Get-Content $log
if ($status -eq 0) { Write-Host "ticky update complete: $installed" -ForegroundColor Green } else { Write-Host "ticky update failed (exit $status). Log: $log" -ForegroundColor Red }
Read-Host 'Press Enter to close'
`, psQuote(req.RepoPath), psQuote(req.TargetCommit), psQuote(req.RecorderBinary), latest)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", err
	}
	return path, nil
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
