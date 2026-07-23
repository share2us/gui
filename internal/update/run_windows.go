//go:build windows

package update

import (
	"fmt"
	"syscall"
	"unsafe"
)

var (
	modshell32       = syscall.NewLazyDLL("shell32.dll")
	procShellExecute = modshell32.NewProc("ShellExecuteW")
)

// LaunchInstaller starts the downloaded installer with a UAC elevation prompt.
//
// The Inno Setup installer is manifested requireAdministrator. os/exec launches
// via CreateProcess, which CANNOT start an elevated-manifest exe — it fails with
// ERROR_ELEVATION_REQUIRED (740), so the installer never appeared. ShellExecuteW
// with the "runas" verb performs the shell-level elevation Windows expects,
// showing the single UAC prompt and then running Setup.exe as admin.
func LaunchInstaller(path string) error {
	verb, err := syscall.UTF16PtrFromString("runas")
	if err != nil {
		return err
	}
	file, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	// ShellExecuteW(hwnd=0, "runas", path, params=nil, dir=nil, SW_SHOWNORMAL).
	// Returns an HINSTANCE cast to int; values > 32 mean success.
	r, _, callErr := procShellExecute.Call(
		0,
		uintptr(unsafe.Pointer(verb)),
		uintptr(unsafe.Pointer(file)),
		0,
		0,
		1, // SW_SHOWNORMAL
	)
	if r <= 32 {
		// callErr is always non-nil for syscall.Call; only meaningful on failure.
		return fmt.Errorf("could not launch the installer (ShellExecute code %d): %w", r, callErr)
	}
	return nil
}
