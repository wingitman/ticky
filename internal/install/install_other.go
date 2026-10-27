//go:build !windows

package install

// Install is Windows-only; other platforms use `make install`.
func Install(Options) error { return ErrUnsupported }

// Uninstall is Windows-only; other platforms use `make uninstall`.
func Uninstall(Options) error { return ErrUnsupported }
