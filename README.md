# Share2Us GUI

A cross-platform desktop app that adds Share2Us to the file manager's right-click
menu (7-Zip style): **right-click a file/folder → `s2u` → `Share`**, then a modal
picks a destination — a public/private link, or a direct end-to-end-encrypted send
to one of your devices or a contact that lands in their Downloads folder.

This is a **GUI front-end only**. It does **not** modify or replace the `s2u`
command-line tool (`share2us/cli`); it reuses the same backend API and the
`share2us/cli-core` library, so behaviour stays identical to the CLI.

Built with [Wails v2](https://wails.io): a Go backend + a WebView UI that is native
on each OS (WebView2 on Windows, WKWebView on macOS, WebKit2GTK on Linux). The app
window and the whole share pipeline are portable; only the file-manager
right-click integration is per-OS.

## Platform support

| Platform | App window | Right-click "Share" integration |
|---|---|---|
| **Windows** | ✅ | ✅ registry cascading verb (Win10 + Win11) |
| **Linux** | ✅ | ✅ KDE/Dolphin ServiceMenu + Nemo action + universal "Open With"; Nautilus native pending |
| **macOS** | ✅ | ⏸ deferred — needs a signed Finder extension (Apple Developer ID) |

The window works everywhere today (drag a file onto it, or "Open with…"); the
file-manager entry lands per-OS and degrades gracefully when absent.

## Setup

The setup scripts always download the **latest release build for your OS/arch**,
verify its checksum, install it, and register the right-click integration.

**Windows** — either download **`Share2Us-Setup-<version>.exe`** from the
[latest release](https://github.com/share2us/gui/releases/latest) and run it (a
normal installer: choose the app, the `s2u` CLI, or both), or use the one-liner
(PowerShell):

```powershell
irm https://raw.githubusercontent.com/share2us/gui/main/scripts/install.ps1 | iex
```

**Linux**:

```sh
curl -fsSL https://raw.githubusercontent.com/share2us/gui/main/scripts/install.sh | sh
```

That's it — then right-click a file/folder → `s2u` → `Share` (on Windows 11, under
"Show more options"), and sign in from the app the first time.

Re-running the setup upgrades to the newest release. Overrides via env vars:
`SHARE2US_GUI_VERSION` (default `latest`), `SHARE2US_GUI_INSTALL_DIR`. The scripts
live in [`scripts/`](scripts/) and pull binaries from GitHub Releases.

> Releases are built by CI for every push to `main` (`.github/workflows/release.yml`)
> and published to GitHub Releases as `share2us-gui_<os>_<arch>.{zip,tar.gz}` with
> `.crc32`/`.sha256` sidecars — the same version model as the CLI (a UTC-timestamp
> `buildVersion`). `share2us-gui --version` prints the installed build.

## Architecture

```
main.go            Verbs: `share <path>`, `--install-shell`, `--uninstall-shell`; opens the window
app.go             Wails-bound methods the modal calls (Status, PendingPaths, ListDevices, Share, InstallShell)
internal/core/     Platform-independent heart — wraps cli-core. Builds & unit-tests on any OS.
  client.go          Load() the saved login (same store as the CLI)
  share.go           ShareLink (public/private) + SendToDevice + SendToContact (sealed-box E2E)
  prepare.go         folder-zip, content-type, sha256, stream-encrypt helpers (ported from the CLI)
  device.go          own-device list → "<name>:<os>" picker
  receive.go         inbox poll → decrypt → save to a destination dir (Downloads)
internal/shell/    File-manager integration behind one interface (Install/Uninstall/Installed):
  shell_windows.go   registry cascading verb
  shell_linux.go     KDE ServiceMenu + Nemo action + "Open With" .desktop (XDG, no admin)
  shell_other.go     no-op fallback (macOS until its integration lands)
frontend/          Vanilla-TS + Vite modal (dark theme), portable across all three OSes
```

Why no `gui-core`: `cli-core` already **is** the shared client SDK (API, auth,
crypto, lanshare, device identity), reused directly here. The thin orchestration
in `internal/core` is GUI-local; if the CLI and GUI ever need identical high-level
flows, promote it up into `cli-core` rather than forking a second core.

The share pipeline mirrors the CLI's `upload()`: create → PUT → complete, with a
fresh AES data key stream-encrypted and sealed per target device (libsodium
sealed-box) for device/contact sends. The **trust model is the existing
backend's**: a contact send only lands if the recipient trusts the sender and has
exposed a device to them.

## Status

Implemented and verified (Linux build + Windows cross-compile + unit tests):

- [x] Project scaffold, wired to `cli-core`
- [x] Windows file-manager integration (registry verb, Win10 + Win11), install/uninstall
- [x] Core share orchestration: public link, private (recipient) link
- [x] Send to own device / to a contact (sealed-box E2E)
- [x] Background receiver: `--receive` polls inbox → decrypt → Downloads, with native toasts (beeep)
- [x] Autostart at login (Windows Run key / Linux XDG autostart) + settings toggles in the modal
- [x] Share modal UI (public / private / device; network tab stubbed)
- [x] In-app sign-in (device-code flow: opens the browser, polls, registers the device key)
- [x] Trust screen: control which of your devices a contact may send to (per-sender device exposure, cli-core v0.5.0)

- [x] Linux file-manager integration: KDE/Dolphin ServiceMenu + Nemo action + "Open With" (XDG, no admin)
- [x] Windows installer (Inno Setup) with GUI / CLI / both component choice + shared login (`installer/`)

Not yet done (see the plan for phasing):

- [ ] Graphical tray icon (menu: open Downloads, pause, quit) around the receiver loop
- [ ] Approval prompts for untrusted senders (pending inbox) surfaced from the tray
- [ ] Nautilus (GNOME) native right-click submenu (needs a python3-nautilus extension)
- [ ] macOS Finder integration (deferred — needs code-signing)
- [ ] Linux packaging (`.deb` / AppImage) with the same GUI/CLI/both choice
- [ ] Background tray receiver + native toasts + autostart + approval prompts
- [ ] Contact device-exposure trust UI + the backend `contact_sender_devices` addition
- [ ] "Send to network" (LAN via `cli-core/lanshare`)
- [ ] Recents, add-device/contact management, installers, code-signing
- [ ] Exact `<name>:<os>` label (needs an `OS` field on `cli-core` `DeviceSession`)

## Building

Requires Go 1.25+, Node, and the Wails CLI
(`go install github.com/wailsapp/wails/v2/cmd/wails@latest`).

```sh
wails build            # native build for the current OS → build/bin/share2us-gui[.exe]
```

Linux additionally needs `libgtk-3-dev` and `libwebkit2gtk-4.0-dev`. The frontend
calls the Go backend via Wails' injected `window.go.main.App`, so `wails build`
regenerates bindings automatically.

### Cross-compiling a check from Linux/CI

The Windows target is CGO-free, so it cross-compiles without a Windows box (this
does not bundle the WebView2 installer — use `wails build` on Windows for a
release):

```sh
cd frontend && npm ci && npm run build && cd ..
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build .
```

### Local development against a sibling `cli-core`

`go.work` (git-ignored) points at `../share2us-cli-core` so you can build against
a local checkout. Released builds use the pinned `github.com/share2us/cli-core`
in `go.mod`.

## Registering the right-click menu (Windows)

```powershell
share2us-gui.exe --install-shell     # adds  s2u ▸ Share  for files and folders (HKCU, no admin)
share2us-gui.exe --uninstall-shell
```
