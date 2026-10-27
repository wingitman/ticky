// Package fsutil holds small filesystem helpers shared by the persistence
// packages.
package fsutil

import (
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// WriteFileAtomic writes data to a uniquely named temp file in the same
// directory as path, syncs it, and renames it over path. Readers therefore see
// either the old or the new content, never a truncated file.
//
// On Windows a rename fails while another process holds the destination open
// (for example a concurrent `ticky --status`), so the rename is retried
// briefly before giving up.
func WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	cleanup := func() { _ = os.Remove(tmp) }

	if _, err := f.Write(data); err != nil {
		f.Close()
		cleanup()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		cleanup()
		return err
	}
	if err := f.Close(); err != nil {
		cleanup()
		return err
	}
	if err := os.Chmod(tmp, perm); err != nil && runtime.GOOS != "windows" {
		cleanup()
		return err
	}
	if err := renameWithRetry(tmp, path); err != nil {
		cleanup()
		return err
	}
	return nil
}

func renameWithRetry(from, to string) error {
	err := os.Rename(from, to)
	if err == nil || runtime.GOOS != "windows" {
		return err
	}
	for i := 0; i < 20; i++ {
		time.Sleep(25 * time.Millisecond)
		if err = os.Rename(from, to); err == nil {
			return nil
		}
	}
	return err
}
