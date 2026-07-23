# MSIX package (Windows Share sheet)

This packages Share2Us as an **MSIX** and declares a **Windows Share Target**, so
S2u appears in the Windows **Share sheet** — the "Share" button in Snip & Sketch,
the Photos app, Explorer, etc.

## What's here

- `AppxManifest.xml` — the package manifest, including the `windows.shareTarget`
  extension (accepts `Bitmap`, `StorageItems`, `Text`, `Uri`).
- `Assets/` — tile/logo images (generated from the S2u brand icon).

## Build (CI)

The release workflow's **`installer`** job (windows-latest) builds it on every
release, signed with the **same** self-signed cert as `Setup.exe` and the app
(the cert Subject `CN=Share2.us, O=Share2.us` must equal the manifest
`Publisher`): `wails build` → stage `share2us-gui.exe` + `Assets/` + a versioned
manifest → `makeappx pack` → `signtool sign`. It publishes the standalone
`Share2Us-<version>.msix` + `Share2Us-Cert.cer`, and **bundles both into
`Setup.exe`**.

## Install

Normally you don't sideload by hand — **`Setup.exe` does it for you.** With the
"Add Share2Us to the Windows Share menu" task ticked (default), the admin
installer trusts the bundled cert (LocalMachine `Root` + `TrustedPublisher` +
`TrustedPeople`) and runs `Add-AppxPackage`, so the publisher shows as
"Share2.us" and S2u appears in the Share sheet. See
[`../windows/sharesheet.ps1`](../windows/sharesheet.ps1).

To sideload the standalone `.msix` manually instead:

```powershell
# As admin: trust the publisher, then install the package.
Import-Certificate -FilePath .\Share2Us-Cert.cer -CertStoreLocation Cert:\LocalMachine\TrustedPeople
Add-AppxPackage .\Share2Us-<version>.msix
```

After install, S2u appears in the Share sheet. (A real CA / Microsoft Store
signature removes the manual cert-trust step.)

## ⚠️ Remaining work — the share-target file hand-off (needs Windows)

Appearing in the Share sheet is done (the manifest). **Receiving the shared file
when the user picks S2u is not yet implemented** — it needs WinRT interop and must
be built and tested on Windows. See the package doc in
[`internal/sharetarget/sharetarget_windows.go`](../../internal/sharetarget/sharetarget_windows.go):

1. Detect the activation via `AppInstance.GetActivatedEventArgs()` →
   `ExtendedActivationKind.ShareTarget`.
2. Read `ShareOperation.Data` (`GetStorageItemsAsync` / `GetBitmapAsync` /
   `GetTextAsync`), write to temp files, call `ReportCompleted()`.
3. Return the paths from `sharetarget.Activation()` — `main.go` already opens the
   share window with them.

Until then, choosing S2u in the Share sheet launches the app but does not receive
the file. **Paste-to-share (Ctrl+V in the app) already covers the screenshot
workflow** in the meantime.
