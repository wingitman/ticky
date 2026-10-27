//go:build !windows

package update

import "errors"

func launchWindows(InstallRequest) error {
	return errors.New("windows updater is only available on Windows")
}
