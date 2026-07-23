//go:build !windows && !linux

// Package shell's fallback build for platforms without file-manager integration
// yet. Windows is implemented (shell_windows.go); Linux (Nautilus / KDE
// ServiceMenu) is next and macOS (a signed Finder extension) is deferred. Until
// those land, these are no-ops so the app still builds and runs everywhere — the
// window works without the right-click entry. When a platform's integration is
// added, give it its own build-tagged file and narrow this tag accordingly.
package shell

import "errors"

// ErrUnsupported is returned by the shell operations on platforms whose
// file-manager integration is not implemented yet.
var ErrUnsupported = errors.New("file-manager integration is not available on this platform yet")

// Install is a no-op off Windows.
func Install(exePath string) error { return ErrUnsupported }

// Uninstall is a no-op off Windows.
func Uninstall() error { return ErrUnsupported }

// Installed always reports false off Windows.
func Installed() bool { return false }
