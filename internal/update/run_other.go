//go:build !windows

package update

import "os/exec"

// LaunchInstaller runs the downloaded artifact directly. Non-Windows platforms
// don't use the installer path (ApplyUpdate opens the release page instead), so
// this exists only to keep the package building for every target.
func LaunchInstaller(path string) error {
	return exec.Command(path).Start()
}
