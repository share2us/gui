//go:build windows

package update

import (
	"syscall"
	"unsafe"
)

// appmodelErrorNoPackage (APPMODEL_ERROR_NO_PACKAGE) is returned by the package
// APIs when the process has no MSIX/AppX package identity.
const appmodelErrorNoPackage = 15700

// runningPackaged reports whether the process has MSIX/AppX package identity —
// i.e. it was installed and launched via the Microsoft Store (or a sideloaded
// MSIX). It calls Win32 GetCurrentPackageFullName with an empty buffer: an
// unpackaged process gets APPMODEL_ERROR_NO_PACKAGE, a packaged one gets
// ERROR_INSUFFICIENT_BUFFER (or success), so "not the no-package error" ==
// packaged. Returns false if the API is unavailable (pre-Windows 8).
func runningPackaged() bool {
	proc := syscall.NewLazyDLL("kernel32.dll").NewProc("GetCurrentPackageFullName")
	if err := proc.Find(); err != nil {
		return false
	}
	var length uint32
	r, _, _ := proc.Call(uintptr(unsafe.Pointer(&length)), 0)
	return r != appmodelErrorNoPackage
}
