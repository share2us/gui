//go:build windows

// Package sharetarget handles Windows Share Target activation — when S2u is
// picked in the Windows Share sheet (Snip & Sketch's "Share", the Photos app,
// Explorer's "Share"). The MSIX manifest (installer/msix/AppxManifest.xml)
// declares the share target; Windows then launches share2us-gui.exe, and this
// package must read the shared items and return them as file paths for the normal
// share flow.
//
// STATUS: NOT YET IMPLEMENTED — this is the piece that must be built and validated
// on Windows. There is no command-line hand-off for a share activation; the data
// only comes through the WinRT ShareOperation API. Steps:
//
//  1. Detect the activation kind:
//     Microsoft.Windows.AppLifecycle.AppInstance.GetActivatedEventArgs()
//     -> ExtendedActivationKind.ShareTarget
//     (Windows App SDK; or the classic Windows.ApplicationModel activation path.)
//  2. From ShareTargetActivatedEventArgs.ShareOperation.Data (a DataPackageView):
//     - GetStorageItemsAsync() -> shared files      (use/copy their paths)
//     - GetBitmapAsync()       -> a shared image     (write to a temp .png)
//     - GetTextAsync()         -> shared text        (write to a temp .txt)
//     then call ShareOperation.ReportCompleted().
//  3. Return the resulting temp file paths.
//
// Suggested Go WinRT binding: github.com/saltosystems/winrt-go (advanced/fragile),
// or a tiny helper exe (C#/C++/WinRT) invoked from Go that prints the temp paths.
package sharetarget

// Activation reports whether this process was launched as a Windows Share Target
// and, if so, the temp file paths of the shared items. Returns (nil, false) until
// the WinRT hand-off documented above is implemented.
func Activation() (paths []string, ok bool) {
	// TODO(windows): implement via the WinRT ShareOperation (see the package doc).
	return nil, false
}
