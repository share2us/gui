//go:build !windows && !linux

// Package autostart fallback for platforms without a login-autostart hook yet
// (e.g. macOS, which will use a LaunchAgent plist).
package autostart

import "errors"

// ErrUnsupported is returned on platforms where autostart is not implemented yet.
var ErrUnsupported = errors.New("autostart is not available on this platform yet")

// Enable is a no-op here.
func Enable(exePath string) error { return ErrUnsupported }

// Disable is a no-op here.
func Disable() error { return ErrUnsupported }

// Enabled always reports false here.
func Enabled() bool { return false }
