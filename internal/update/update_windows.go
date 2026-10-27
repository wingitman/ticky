//go:build windows

package update

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
)

// launchWindows writes the updater for the chosen shell and opens it in a
// new console window.
func launchWindows(req InstallRequest) error {
	shell := chooseWindowsShell(req.WindowsShell, powerShellUsable)
	if shell == shellPowerShell {
		script, err := writeWindowsScript(req)
		if err != nil {
			return err
		}
		if req.Terminal != "" {
			return exec.Command(req.Terminal, "powershell.exe", "-NoExit", "-ExecutionPolicy", "Bypass", "-File", script).Start()
		}
		if _, err := exec.LookPath("wt.exe"); err == nil {
			return exec.Command("wt.exe", "powershell.exe", "-NoExit", "-ExecutionPolicy", "Bypass", "-File", script).Start()
		}
		return startNewConsole(exec.Command("powershell.exe", "-NoExit", "-ExecutionPolicy", "Bypass", "-File", script))
	}

	script, err := writeCmdScript(req)
	if err != nil {
		return err
	}
	if req.Terminal != "" {
		// Terminals such as wt.exe run a .cmd path through cmd.exe themselves.
		return exec.Command(req.Terminal, script).Start()
	}
	comspec := comSpec()
	cmd := exec.Command(comspec)
	// cmd.exe does not parse arguments the way Go quotes them. With /c and a
	// leading quote it strips exactly the outermost pair, so wrap the quoted
	// script path in one more pair of quotes.
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CmdLine: fmt.Sprintf(`"%s" /d /c ""%s""`, comspec, script),
	}
	return startNewConsole(cmd)
}

func startNewConsole(cmd *exec.Cmd) error {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags |= windows.CREATE_NEW_CONSOLE
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

func comSpec() string {
	if c := os.Getenv("ComSpec"); c != "" {
		return c
	}
	return filepath.Join(os.Getenv("SystemRoot"), "System32", "cmd.exe")
}

// powerShellUsable runs a tiny script with the same flags the updater uses.
// It fails when powershell.exe is blocked (AppLocker/SRP), when Group Policy
// forces an execution policy that overrides -ExecutionPolicy Bypass, or when
// PowerShell runs in Constrained Language Mode.
func powerShellUsable() bool {
	exe, err := exec.LookPath("powershell.exe")
	if err != nil {
		return false
	}
	dir, err := updatesDir()
	if err != nil {
		return false
	}
	probe := filepath.Join(dir, fmt.Sprintf("probe-%d.ps1", time.Now().UnixNano()))
	if err := os.WriteFile(probe, []byte(powerShellProbeScript), 0644); err != nil {
		return false
	}
	defer os.Remove(probe)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, exe, "-NoLogo", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", probe)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NO_WINDOW, HideWindow: true}
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "FullLanguage"
}
