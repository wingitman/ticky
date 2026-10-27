// Package install implements `ticky --install` and `ticky --uninstall`.
//
// The Windows installers (install.cmd, install.ps1) are thin wrappers that
// build or locate ticky.exe and then hand over to this package, so Command
// Prompt and PowerShell installs behave identically and neither shell needs
// any real logic. The helpers in this file are platform-neutral so they can
// be unit-tested anywhere; the registry and filesystem side effects live in
// install_windows.go.
package install

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Options configures an install or uninstall run.
type Options struct {
	// From is the ticky executable to install. Defaults to the running binary.
	From string
	// DesktopFrom is an optional ticky-desktop executable. Defaults to a
	// ticky-desktop binary next to From, when present.
	DesktopFrom string
	// Dir overrides the install directory.
	Dir string
	// Update skips shell integration and first-install guidance; used by the
	// detached updater.
	Update bool
	// PowerShellProfile, when set, is the $PROFILE path to add the prompt
	// integration to. Only install.ps1 passes it.
	PowerShellProfile string
	// Out receives progress output. Defaults to os.Stdout.
	Out io.Writer
}

func (o Options) out() io.Writer {
	if o.Out == nil {
		return os.Stdout
	}
	return o.Out
}

// ErrUnsupported is returned on platforms that install via `make install`.
var ErrUnsupported = errors.New("ticky --install is only used on Windows; run `make install` instead")

// IntegrationMarker identifies blocks ticky adds to user shell config files.
const IntegrationMarker = "# ticky shell integration"

// powerShellPromptLines is the prompt block added to a PowerShell profile.
var powerShellPromptLines = []string{
	IntegrationMarker,
	"function prompt {",
	"    $s = ticky --status 2>$null",
	`    if ($s) { Write-Host "$s " -NoNewline -ForegroundColor Blue }`,
	`    "PS $(Get-Location)> "`,
	"}",
}

// PowerShellPromptBlock returns the PowerShell profile integration block.
func PowerShellPromptBlock() string {
	return strings.Join(powerShellPromptLines, "\n") + "\n"
}

var powerShellBlockRe = regexp.MustCompile(blockPattern(powerShellPromptLines))

// tmuxBlockRe matches the tmux block written by any ticky installer.
var tmuxBlockRe = regexp.MustCompile(`(?m)(^\r?\n)?^` + regexp.QuoteMeta(IntegrationMarker) +
	`\r?\nset -g status-interval[^\n]*\nset -g status-right-length[^\n]*\nset -g status-right[^\n]*(\n|\z)`)

// blockPattern matches lines as a whole block, with either line ending,
// plus one preceding blank line (which AddBlock inserts as a separator).
func blockPattern(lines []string) string {
	quoted := make([]string, len(lines))
	for i, l := range lines {
		quoted[i] = regexp.QuoteMeta(l)
	}
	return `(?m)(^\r?\n)?^` + strings.Join(quoted, `\r?\n`) + `(\r?\n|\z)`
}

// TmuxBlock returns the tmux status-bar integration block.
func TmuxBlock() string {
	return "\n" + IntegrationMarker + "\n" +
		"set -g status-interval 1\n" +
		"set -g status-right-length 120\n" +
		`set -g status-right "#(ticky --status 2>/dev/null)  #[fg=blue]#{?client_prefix,PREFIX ,}#[fg=brightblack]#h "` + "\n"
}

// AddBlock appends block to content, separated by a blank line, unless an
// integration marker is already present. The file's existing line ending
// style is kept. It reports whether content changed.
func AddBlock(content, block string) (string, bool) {
	if strings.Contains(content, IntegrationMarker) {
		return content, false
	}
	nl := lineEnding(content)
	block = strings.TrimLeft(block, "\r\n")
	if nl == "\r\n" {
		block = strings.ReplaceAll(block, "\n", "\r\n")
	}
	if content != "" {
		if !strings.HasSuffix(content, "\n") {
			content += nl
		}
		content += nl
	}
	return content + block, true
}

// RemovePowerShellBlock removes the PowerShell prompt block.
func RemovePowerShellBlock(content string) (string, bool) {
	out := powerShellBlockRe.ReplaceAllString(content, "")
	return out, out != content
}

// RemoveTmuxBlock removes the tmux status-bar block.
func RemoveTmuxBlock(content string) (string, bool) {
	out := tmuxBlockRe.ReplaceAllString(content, "")
	return out, out != content
}

func lineEnding(content string) string {
	if strings.Contains(content, "\r\n") {
		return "\r\n"
	}
	return "\n"
}

// normalizePathEntry makes PATH entries comparable: trimmed, without a
// trailing separator, and case-folded (Windows paths are case-insensitive).
func normalizePathEntry(entry string, expand func(string) string) string {
	entry = strings.TrimSpace(strings.Trim(strings.TrimSpace(entry), `"`))
	if expand != nil {
		entry = expand(entry)
	}
	entry = strings.TrimRight(entry, `\/`)
	return strings.ToLower(entry)
}

// AddToPathList appends dir to a semicolon-separated PATH value unless an
// equivalent entry is already present.
func AddToPathList(list, dir string, expand func(string) string) (string, bool) {
	want := normalizePathEntry(dir, expand)
	for _, entry := range strings.Split(list, ";") {
		if entry != "" && normalizePathEntry(entry, expand) == want {
			return list, false
		}
	}
	list = strings.TrimRight(list, ";")
	if list == "" {
		return dir, true
	}
	return list + ";" + dir, true
}

// RemoveFromPathList removes every entry equivalent to dir from a
// semicolon-separated PATH value, and drops empty entries.
func RemoveFromPathList(list, dir string, expand func(string) string) (string, bool) {
	want := normalizePathEntry(dir, expand)
	parts := strings.Split(list, ";")
	kept := parts[:0]
	removed := false
	for _, entry := range parts {
		if entry == "" {
			continue
		}
		if normalizePathEntry(entry, expand) == want {
			removed = true
			continue
		}
		kept = append(kept, entry)
	}
	if !removed {
		return list, false
	}
	return strings.Join(kept, ";"), true
}

// ReplaceExecutable installs src at dst. Windows refuses to overwrite a
// running executable but allows renaming it, so an existing dst is first
// moved to dst+".old" (removed by CleanupOld on a later start). The new file
// is copied to a temp name next to dst and renamed into place; on failure
// the previous executable is restored.
func ReplaceExecutable(src, dst string) error {
	if same, _ := sameFile(src, dst); same {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	tmp := dst + ".new"
	_ = os.Remove(tmp)
	if err := copyFile(src, tmp); err != nil {
		_ = os.Remove(tmp)
		return err
	}

	old := dst + ".old"
	_ = os.Remove(old) // fails harmlessly if a previous .old is still running
	movedOld := false
	if _, err := os.Stat(dst); err == nil {
		if err := os.Rename(dst, old); err != nil {
			// A locked .old can block the rename; fall back to a unique name.
			old = fmt.Sprintf("%s.%d.old", dst, os.Getpid())
			if err := os.Rename(dst, old); err != nil {
				_ = os.Remove(tmp)
				return fmt.Errorf("move existing %s aside: %w", dst, err)
			}
		}
		movedOld = true
	}
	if err := os.Rename(tmp, dst); err != nil {
		if movedOld {
			_ = os.Rename(old, dst)
		}
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// CleanupOld removes executables left behind by ReplaceExecutable. Files that
// are still running cannot be removed and are retried on a later start.
func CleanupOld(exe string) {
	if exe == "" {
		return
	}
	matches, _ := filepath.Glob(exe + "*.old")
	for _, m := range matches {
		_ = os.Remove(m)
	}
}

func sameFile(a, b string) (bool, error) {
	ai, err := os.Stat(a)
	if err != nil {
		return false, err
	}
	bi, err := os.Stat(b)
	if err != nil {
		return false, err
	}
	return os.SameFile(ai, bi), nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Sync(); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// desktopSibling returns the ticky-desktop binary next to exe, if present.
func desktopSibling(exe string) string {
	dir := filepath.Dir(exe)
	ext := filepath.Ext(exe)
	candidate := filepath.Join(dir, "ticky-desktop"+ext)
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	return ""
}

// editFile applies fn to the file at path, writing only when it changes.
// Missing files are skipped. It reports whether the file changed.
func editFile(path string, fn func(string) (string, bool)) (bool, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	updated, changed := fn(string(data))
	if !changed {
		return false, nil
	}
	return true, os.WriteFile(path, []byte(updated), 0o644)
}
