package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAddToPathList(t *testing.T) {
	dir := `C:\Users\x\AppData\Local\Programs\ticky`
	cases := []struct {
		in, want string
		changed  bool
	}{
		{"", dir, true},
		{`C:\bin`, `C:\bin;` + dir, true},
		{`C:\bin;`, `C:\bin;` + dir, true},
		{`C:\bin;c:\users\X\appdata\local\programs\TICKY\`, `C:\bin;c:\users\X\appdata\local\programs\TICKY\`, false},
		{`"` + dir + `";C:\bin`, `"` + dir + `";C:\bin`, false},
	}
	for _, c := range cases {
		got, changed := AddToPathList(c.in, dir, nil)
		if got != c.want || changed != c.changed {
			t.Errorf("AddToPathList(%q) = %q, %v; want %q, %v", c.in, got, changed, c.want, c.changed)
		}
	}
}

func TestAddToPathListExpandsVariables(t *testing.T) {
	expand := func(s string) string {
		return strings.ReplaceAll(s, "%LOCALAPPDATA%", `C:\Users\x\AppData\Local`)
	}
	in := `%LOCALAPPDATA%\Programs\ticky;C:\bin`
	if _, changed := AddToPathList(in, `C:\Users\x\AppData\Local\Programs\ticky`, expand); changed {
		t.Fatal("expanded entry should match")
	}
}

func TestRemoveFromPathList(t *testing.T) {
	dir := `C:\Programs\ticky`
	got, changed := RemoveFromPathList(`C:\a;;c:\programs\ticky\;C:\b;C:\Programs\ticky`, dir, nil)
	if !changed || got != `C:\a;C:\b` {
		t.Fatalf("got %q, %v", got, changed)
	}
	if _, changed := RemoveFromPathList(`C:\a;C:\b`, dir, nil); changed {
		t.Fatal("expected no change")
	}
}

func TestPowerShellBlockRoundTrip(t *testing.T) {
	for _, nl := range []string{"\n", "\r\n"} {
		original := strings.ReplaceAll("Set-Alias ll Get-ChildItem\n# keep me\n", "\n", nl)
		added, changed := AddBlock(original, PowerShellPromptBlock())
		if !changed || !strings.Contains(added, IntegrationMarker) {
			t.Fatalf("block not added")
		}
		if nl == "\r\n" && strings.Count(added, "\n") != strings.Count(added, "\r\n") {
			t.Fatalf("mixed line endings: %q", added)
		}
		if _, again := AddBlock(added, PowerShellPromptBlock()); again {
			t.Fatal("block added twice")
		}
		removed, changed := RemovePowerShellBlock(added)
		if !changed || removed != original {
			t.Fatalf("round trip mismatch:\n%q\n%q", removed, original)
		}
	}
}

func TestRemovePowerShellBlockFromLegacyInstall(t *testing.T) {
	// install.ps1 used Add-Content with a here-string starting with a newline.
	legacy := "Import-Module posh-git\r\n\r\n" + strings.ReplaceAll(PowerShellPromptBlock(), "\n", "\r\n") + "\r\nWrite-Host after\r\n"
	got, changed := RemovePowerShellBlock(legacy)
	if !changed || strings.Contains(got, "ticky") {
		t.Fatalf("legacy block not removed: %q", got)
	}
	if !strings.Contains(got, "posh-git") || !strings.Contains(got, "Write-Host after") {
		t.Fatalf("user content lost: %q", got)
	}
}

func TestTmuxBlockRoundTrip(t *testing.T) {
	original := "set -g mouse on\n"
	added, changed := AddBlock(original, TmuxBlock())
	if !changed {
		t.Fatal("not added")
	}
	removed, changed := RemoveTmuxBlock(added)
	if !changed || removed != original {
		t.Fatalf("got %q", removed)
	}
	legacy := original + "\n# ticky shell integration\nset -g status-interval 1\nset -g status-right-length 120\nset -g status-right \"#(ticky --status 2>$null)  #h \"\n"
	if got, changed := RemoveTmuxBlock(legacy); !changed || got != original {
		t.Fatalf("legacy tmux block: %q", got)
	}
}

func TestReplaceExecutable(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "build", "ticky.exe")
	dst := filepath.Join(dir, "install", "ticky.exe")
	if err := os.MkdirAll(filepath.Dir(src), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(src, []byte("v1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := ReplaceExecutable(src, dst); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(src, []byte("v2"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := ReplaceExecutable(src, dst); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(dst); string(got) != "v2" {
		t.Fatalf("dst = %q", got)
	}
	if got, _ := os.ReadFile(dst + ".old"); string(got) != "v1" {
		t.Fatalf("old = %q", got)
	}
	if _, err := os.Stat(dst + ".new"); !os.IsNotExist(err) {
		t.Fatal("temp file left behind")
	}
	CleanupOld(dst)
	if _, err := os.Stat(dst + ".old"); !os.IsNotExist(err) {
		t.Fatal(".old not cleaned up")
	}
	// Installing a binary onto itself is a no-op.
	if err := ReplaceExecutable(dst, dst); err != nil {
		t.Fatal(err)
	}
}
