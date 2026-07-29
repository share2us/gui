//go:build store

package update

import "errors"

// LaunchInstaller never runs in the Store build: the Microsoft Store applies
// updates itself, so there is no downloaded installer to launch. This stub keeps
// the package compiling without the installer-launch machinery and fails closed
// if it were ever reached.
func LaunchInstaller(string) error {
	return errors.New("updates are managed by the Microsoft Store")
}
