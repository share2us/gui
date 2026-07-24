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
release: `wails build` → stage `share2us-gui.exe` + `Assets/` + a versioned
manifest → `makeappx pack`. It publishes the standalone `Share2Us-<version>.msix`
as a release asset for **Microsoft Store submission** (the Store re-signs it with
a trusted cert; its `Publisher` identity is assigned by Partner Center).

## Distribution — via the Microsoft Store (NOT the installer)

The Share-sheet MSIX ships through the **Microsoft Store**. `Setup.exe` no longer
installs it.

Why the change: the installer used to sideload the MSIX by trusting a bundled
**self-signed** cert into `LocalMachine\Root`. "An installer adds a root CA
certificate" is a textbook malware behavior — Windows Defender and Google Safe
Browsing hard-block it ("dangerous/virus"). The Store signs the package with a
trusted cert, so there is no root-store tampering and no self-signed anything.

The published `.msix` is **unsigned** (the Store signs it on ingestion), so it
can't be sideloaded as-is. For local dev testing, re-sign your own copy with a
self-signed cert whose Subject equals the manifest `Publisher`, trust that cert
into `TrustedPeople` (never `Root`), then `Add-AppxPackage`. End users get it from
the Store.

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
