//go:build windows

package install

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// DefaultDir is the per-user install directory (no admin rights required).
func DefaultDir() (string, error) {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		var err error
		base, err = windows.KnownFolderPath(windows.FOLDERID_LocalAppData, 0)
		if err != nil {
			return "", fmt.Errorf("cannot resolve %%LOCALAPPDATA%%: %w", err)
		}
	}
	return filepath.Join(base, "Programs", "ticky"), nil
}

// Install copies ticky into the install directory, adds it to the user PATH,
// and (unless Update is set) adds optional shell integration.
func Install(opts Options) error {
	w := opts.out()
	src := opts.From
	if src == "" {
		exe, err := os.Executable()
		if err != nil {
			return err
		}
		src = exe
	}
	src, err := filepath.Abs(src)
	if err != nil {
		return err
	}
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("source binary: %w", err)
	}
	dir := opts.Dir
	if dir == "" {
		if dir, err = DefaultDir(); err != nil {
			return err
		}
	}

	step(w, "Installing binary...")
	dst := filepath.Join(dir, "ticky.exe")
	if err := ReplaceExecutable(src, dst); err != nil {
		return fmt.Errorf("install %s: %w", dst, err)
	}
	ok(w, "Installed: "+dst)

	desktop := opts.DesktopFrom
	if desktop == "" {
		desktop = desktopSibling(src)
	}
	if desktop != "" {
		desktopDst := filepath.Join(dir, "ticky-desktop.exe")
		if err := ReplaceExecutable(desktop, desktopDst); err != nil {
			note(w, "Could not install ticky-desktop.exe: "+err.Error())
		} else {
			ok(w, "Installed: "+desktopDst)
		}
	}

	step(w, "Updating user PATH...")
	added, err := editUserPath(func(list string) (string, bool) {
		return AddToPathList(list, dir, expandEnv)
	})
	switch {
	case err != nil:
		note(w, "Could not update PATH: "+err.Error())
		note(w, "Add this folder to your PATH manually: "+dir)
	case added:
		ok(w, "Added "+dir+" to user PATH.")
	default:
		note(w, dir+" is already in your PATH.")
	}

	if !opts.Update {
		shellIntegration(w, opts.PowerShellProfile)
	}

	cfgDir := filepath.Join(os.Getenv("APPDATA"), "delbysoft")
	fmt.Fprintln(w)
	if opts.Update {
		fmt.Fprintln(w, "  ticky updated successfully!")
		return nil
	}
	fmt.Fprintln(w, "  ticky installed successfully!")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  Open a new Command Prompt or PowerShell window and run:")
	fmt.Fprintln(w, "    ticky")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  Config file (created on first launch):")
	fmt.Fprintln(w, "    "+filepath.Join(cfgDir, "ticky.toml"))
	fmt.Fprintln(w, "  Task data file (created on first task):")
	fmt.Fprintln(w, "    "+filepath.Join(cfgDir, "tasks.toml"))
	return nil
}

// Uninstall removes ticky from the user PATH, removes shell integration, and
// deletes the install directory. When the running binary lives in that
// directory, deletion is handed to a short-lived cmd.exe that waits for this
// process to exit first.
func Uninstall(opts Options) error {
	w := opts.out()
	dir := opts.Dir
	if dir == "" {
		var err error
		if dir, err = DefaultDir(); err != nil {
			return err
		}
	}

	step(w, "Updating user PATH...")
	removed, err := editUserPath(func(list string) (string, bool) {
		return RemoveFromPathList(list, dir, expandEnv)
	})
	switch {
	case err != nil:
		note(w, "Could not update PATH: "+err.Error())
	case removed:
		ok(w, "Removed "+dir+" from user PATH.")
	default:
		note(w, dir+" was not in user PATH.")
	}

	step(w, "Removing shell prompt integration...")
	for _, p := range powerShellProfiles() {
		if changed, err := editFile(p, RemovePowerShellBlock); err != nil {
			note(w, "Could not edit "+p+": "+err.Error())
		} else if changed {
			ok(w, "Removed ticky integration from "+p)
		}
	}
	if p := tmuxConfig(); p != "" {
		if changed, err := editFile(p, RemoveTmuxBlock); err != nil {
			note(w, "Could not edit "+p+": "+err.Error())
		} else if changed {
			ok(w, "Removed ticky integration from "+p)
		}
	}

	step(w, "Removing ticky binary...")
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		note(w, "Install directory not found (already removed?): "+dir)
	} else if insideDir(dir) {
		if err := deleteAfterExit(dir); err != nil {
			note(w, "Could not schedule removal, delete it manually: "+dir)
		} else {
			ok(w, "Removing "+dir+" after ticky exits.")
		}
	} else if err := os.RemoveAll(dir); err != nil {
		note(w, "Could not remove "+dir+": "+err.Error())
	} else {
		ok(w, "Removed: "+dir)
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "  ticky has been uninstalled.")
	fmt.Fprintln(w, "  Your config and task data have been left in place:")
	fmt.Fprintln(w, "    "+filepath.Join(os.Getenv("APPDATA"), "delbysoft"))
	return nil
}

func shellIntegration(w io.Writer, profile string) {
	step(w, "Setting up shell prompt integration...")
	if p := tmuxConfig(); p != "" {
		if changed, err := editFile(p, func(s string) (string, bool) { return AddBlock(s, TmuxBlock()) }); err != nil {
			note(w, "tmux: could not edit "+p+": "+err.Error())
		} else if changed {
			ok(w, "tmux: added live status bar to "+p)
		} else {
			note(w, "tmux: ticky already integrated - skipping.")
		}
		return
	}
	if profile == "" {
		note(w, "Command Prompt cannot show ticky status in its prompt.")
		note(w, "Run 'ticky --status' or 'ticky --check' to see the active timer.")
		return
	}
	data, err := os.ReadFile(profile)
	if err != nil && !os.IsNotExist(err) {
		note(w, "PowerShell: could not read "+profile+": "+err.Error())
		return
	}
	updated, changed := AddBlock(string(data), PowerShellPromptBlock())
	if !changed {
		note(w, "PowerShell profile: ticky already integrated - skipping.")
		return
	}
	if err := os.MkdirAll(filepath.Dir(profile), 0o755); err != nil {
		note(w, "PowerShell: "+err.Error())
		return
	}
	if err := os.WriteFile(profile, []byte(updated), 0o644); err != nil {
		note(w, "PowerShell: "+err.Error())
		return
	}
	ok(w, "PowerShell: added ticky prompt to "+profile)
}

// editUserPath applies fn to HKCU\Environment\Path, keeping the value type
// (REG_EXPAND_SZ entries such as %USERPROFILE% must stay unexpanded), and
// broadcasts the change so new consoles see it without signing out. setx is
// deliberately avoided: it truncates PATH to 1024 characters.
func editUserPath(fn func(string) (string, bool)) (bool, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, "Environment", registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return false, err
	}
	defer k.Close()

	current, valType, err := k.GetStringValue("Path")
	if errors.Is(err, registry.ErrNotExist) {
		current, valType = "", registry.EXPAND_SZ
	} else if err != nil {
		return false, err
	}
	updated, changed := fn(current)
	if !changed {
		return false, nil
	}
	if valType == registry.SZ {
		err = k.SetStringValue("Path", updated)
	} else {
		err = k.SetExpandStringValue("Path", updated)
	}
	if err != nil {
		return false, err
	}
	broadcastEnvironmentChange()
	return true, nil
}

func broadcastEnvironmentChange() {
	const (
		hwndBroadcast   = 0xffff
		wmSettingChange = 0x001A
		smtoAbortIfHung = 0x0002
	)
	env, err := windows.UTF16PtrFromString("Environment")
	if err != nil {
		return
	}
	proc := windows.NewLazySystemDLL("user32.dll").NewProc("SendMessageTimeoutW")
	if proc.Find() != nil {
		return
	}
	var result uintptr
	_, _, _ = proc.Call(hwndBroadcast, wmSettingChange, 0, uintptr(unsafe.Pointer(env)),
		smtoAbortIfHung, 5000, uintptr(unsafe.Pointer(&result)))
}

func expandEnv(s string) string {
	if out, err := registry.ExpandString(s); err == nil {
		return out
	}
	return s
}

// powerShellProfiles lists the CurrentUserCurrentHost profiles for Windows
// PowerShell 5.1 and PowerShell 7. Documents is resolved through the shell
// known-folder API so OneDrive-redirected folders are found.
func powerShellProfiles() []string {
	docs, err := windows.KnownFolderPath(windows.FOLDERID_Documents, 0)
	if err != nil || docs == "" {
		docs = filepath.Join(os.Getenv("USERPROFILE"), "Documents")
	}
	return []string{
		filepath.Join(docs, "WindowsPowerShell", "Microsoft.PowerShell_profile.ps1"),
		filepath.Join(docs, "PowerShell", "Microsoft.PowerShell_profile.ps1"),
	}
}

func tmuxConfig() string {
	home := os.Getenv("USERPROFILE")
	for _, p := range []string{
		filepath.Join(home, ".config", "tmux", "tmux.conf"),
		filepath.Join(home, ".tmux.conf"),
	} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func insideDir(dir string) bool {
	exe, err := os.Executable()
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(dir, exe)
	return err == nil && !strings.HasPrefix(rel, "..")
}

// deleteAfterExit removes dir once this process has exited. A running
// executable cannot delete itself, so a hidden cmd.exe waits ~2 seconds and
// then removes the folder.
func deleteAfterExit(dir string) error {
	comspec := os.Getenv("ComSpec")
	if comspec == "" {
		comspec = filepath.Join(os.Getenv("SystemRoot"), "System32", "cmd.exe")
	}
	cmd := exec.Command(comspec)
	// cmd.exe does not follow the argv quoting rules Go uses, so build the
	// raw command line: /c "<commands>" strips exactly the outer quotes.
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CmdLine:       fmt.Sprintf(`"%s" /d /c "ping -n 3 127.0.0.1 >nul & rmdir /s /q "%s""`, comspec, dir),
		CreationFlags: windows.CREATE_NO_WINDOW,
		HideWindow:    true,
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

func step(w io.Writer, msg string) { fmt.Fprintln(w, "==> "+msg) }
func ok(w io.Writer, msg string)   { fmt.Fprintln(w, "    "+msg) }
func note(w io.Writer, msg string) { fmt.Fprintln(w, "    "+msg) }
