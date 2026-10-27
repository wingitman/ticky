package update

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestIsTickyRemote(t *testing.T) {
	cases := map[string]bool{
		"https://github.com/wingitman/ticky.git": true,
		"https://github.com/wingitman/ticky":     true,
		"git@github.com:wingitman/ticky.git":     true,
		"git@github.com:someone/ticky":           true,
		"https://github.com/wingitman/other.git": false,
	}
	for remote, want := range cases {
		if got := isTickyRemote(remote); got != want {
			t.Fatalf("isTickyRemote(%q) = %v, want %v", remote, got, want)
		}
	}
}

func TestWriteUnixScriptKeepsFailureVisible(t *testing.T) {
	dir := t.TempDir()
	path, err := writeUnixScript(InstallRequest{
		RepoPath:       dir,
		TargetCommit:   "abc123",
		RecorderBinary: "/tmp/ticky",
	})
	if err != nil {
		t.Fatal(err)
	}

	if out, err := exec.Command("sh", "-n", path).CombinedOutput(); err != nil {
		t.Fatalf("generated script is invalid: %v\n%s", err, out)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	script := string(content)
	for _, want := range []string{
		"ticky update failed",
		"Press Enter to close",
		"make install UPDATE=1",
		"worktree add --detach",
		"BUILD_DIR=\"$work/build\"",
		"updater output",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("generated script does not contain %q", want)
		}
	}
}

func TestWriteWindowsScriptKeepsFailureVisible(t *testing.T) {
	path, err := writeWindowsScript(InstallRequest{RepoPath: `C:\ticky`, TargetCommit: "abc123"})
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	script := string(content)
	for _, want := range []string{"$ErrorActionPreference = 'Continue'", "worktree add --detach", "'--install', '--update', '--from'", "--record-update", "ticky update failed", "Read-Host 'Press Enter to close'"} {
		if !strings.Contains(script, want) {
			t.Errorf("generated PowerShell script does not contain %q", want)
		}
	}
}

func TestWriteCmdScript(t *testing.T) {
	path, err := writeCmdScript(InstallRequest{
		RepoPath:       `C:\Users\A & B\100%\ticky!`,
		TargetCommit:   "abc123",
		RecorderBinary: `C:\Users\A & B\AppData\Local\Programs\ticky\ticky.exe`,
	})
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	script := string(content)
	if strings.Count(script, "\n") != strings.Count(script, "\r\n") {
		t.Fatal("batch files must use CRLF line endings")
	}
	for _, want := range []string{
		"setlocal EnableExtensions DisableDelayedExpansion",
		`set "REPO=C:\Users\A & B\100%%\ticky!"`,
		`set "TARGET=abc123"`,
		`set "LATEST=0"`,
		`git -C "%REPO%" worktree add --detach "%WORK%" "%TARGET%"`,
		`"%INSTALLER%" --install --update --from "%BUILT%"`,
		`--record-update --update-commit "%COMMIT%"`,
		"for /f \"usebackq delims=\" %%c in (`git -C \"%WORK%\" rev-parse HEAD 2^>nul`) do set \"COMMIT=%%c\"",
		"ticky update failed",
		"pause >nul",
		":resolve_latest",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("generated cmd script does not contain %q", want)
		}
	}
}

func TestCmdSet(t *testing.T) {
	cases := map[string]string{
		`C:\plain`:      `set "X=C:\plain"`,
		`50% & "x"!^`:   `set "X=50%% & x!^"`,
		"line\r\nbreak": `set "X=linebreak"`,
	}
	for in, want := range cases {
		if got := cmdSet("X", in); got != want {
			t.Errorf("cmdSet(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestChooseWindowsShell(t *testing.T) {
	yes := func() bool { return true }
	no := func() bool { return false }
	probed := false
	spy := func() bool { probed = true; return true }
	cases := []struct {
		pref  string
		probe func() bool
		want  string
	}{
		{"auto", yes, shellPowerShell},
		{"auto", no, shellCmd},
		{"", no, shellCmd},
		{"bogus", yes, shellPowerShell},
		{"cmd", yes, shellCmd},
		{"PowerShell", no, shellPowerShell},
		{"auto", nil, shellCmd},
	}
	for _, c := range cases {
		if got := chooseWindowsShell(c.pref, c.probe); got != c.want {
			t.Errorf("chooseWindowsShell(%q) = %q, want %q", c.pref, got, c.want)
		}
	}
	if chooseWindowsShell("cmd", spy); probed {
		t.Error("explicit cmd must not probe PowerShell")
	}
}
