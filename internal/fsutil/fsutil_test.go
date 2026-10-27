package fsutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteFileAtomicReplacesAndCleansUp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.toml")
	if err := WriteFileAtomic(path, []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteFileAtomic(path, []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "two" {
		t.Fatalf("content = %q, want two", got)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("expected only the target file, found %d entries", len(entries))
	}
}

func TestWriteFileAtomicCreatesDir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "x.toml")
	if err := WriteFileAtomic(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}
