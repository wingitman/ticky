//go:build !desktop

package desktop

import "errors"

// Run is unavailable in the portable CLI build. Native builds include the
// Ebiten frontend with -tags desktop.
func Run() error {
	return errors.New("desktop frontend is not included in this build; install ticky-desktop or build with -tags desktop")
}
