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
		"git checkout \"$restore_ref\"",
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
	for _, want := range []string{"$ErrorActionPreference = 'Continue'", "ticky update failed", "Read-Host 'Press Enter to close'"} {
		if !strings.Contains(script, want) {
			t.Errorf("generated PowerShell script does not contain %q", want)
		}
	}
}
