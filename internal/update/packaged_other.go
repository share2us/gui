//go:build !windows

package update

// runningPackaged is always false off Windows: MSIX package identity is a Windows
// concept and the Microsoft Store distribution only applies to the Windows build.
func runningPackaged() bool { return false }
