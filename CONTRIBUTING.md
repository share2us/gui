# Working on the desktop app

The user-facing documentation is at [docs.share2.us](https://docs.share2.us) and
the README is written for someone who wants to install the app. This file is for
someone changing it.

## What it is built from

[Wails v2](https://wails.io): a Go backend and a WebView interface that is native
on each platform (WebView2 on Windows, WKWebView on macOS, WebKit2GTK on Linux).
The window and the whole share pipeline are portable; only the file-manager
integration is per-platform.

It is a front end. It does not modify or replace the `s2u` command-line tool. Both
use the same backend and the same `share2us/cli-core` library, which is why they
behave identically.

```
main.go            Verbs: `share <path>`, `--install-shell`, `--uninstall-shell`; opens the window
app.go             The methods the interface calls, bound by Wails
internal/core/     Platform-independent heart, wrapping cli-core. Builds and tests on any OS.
  client.go          Loads the saved login, from the same store the CLI uses
  share.go           Link shares, plus device and contact sends (sealed-box end to end)
  prepare.go         Folder zipping, content type, checksums, stream encryption
  device.go          The account's own devices, for the picker
  receive.go         Inbox poll, decrypt, and place the file
internal/shell/    File-manager integration behind one interface
  shell_windows.go   Registry cascading verb
  shell_linux.go     KDE ServiceMenu, Nemo action, and an "Open With" desktop entry
  shell_other.go     No-op, until macOS lands
internal/update/   The updater. Compiled out of the Store build (`-tags store`).
frontend/          A single TypeScript file and a stylesheet, built by Vite
```

**Why there is no `gui-core`.** `cli-core` already is the shared client library:
API, auth, crypto, lanshare, device identity. The orchestration in
`internal/core` is specific to this app. If the two clients ever need the same
high-level flow, promote it into `cli-core` rather than forking a second core.

## Building

Needs Go 1.25 or newer, Node, and the Wails CLI:

```sh
go install github.com/wailsapp/wails/v2/cmd/wails@latest
wails build          # → build/bin/share2us-gui
```

Linux also needs `libgtk-3-dev` and `libwebkit2gtk-4.0-dev`. The frontend reaches
the Go side through Wails' injected `window.go.main.App`, and `wails build`
regenerates those bindings.

### A Windows check from Linux

The Windows target is free of cgo, so it cross-compiles. This does not bundle the
WebView2 installer, so use `wails build` on Windows for anything you intend to
ship:

```sh
cd frontend && npm ci && npm run build && cd ..
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build .
```

### Against a local cli-core

`go.work` (git-ignored) points at `../share2us-cli-core`, so a local checkout is
what you build against. Released builds use the version pinned in `go.mod`.

### The right-click menu

```powershell
share2us-gui.exe --install-shell     # adds  s2u ▸ Share, per-user, no admin
share2us-gui.exe --uninstall-shell
```

## Screenshots

`docs/screenshots/` holds the images the README uses. They are the real interface
with mocked data, not drawings. See the note beside them for how to regenerate
after a change to the window.

## Releases

A merge to `main` that touches Go, the frontend, the build or the installer cuts a
**stable** release, and it takes its notes from `CHANGELOG.md`'s `[Unreleased]`
section. An empty section ships nothing, which is how a refactor avoids bothering
anyone. `workflow_dispatch` cuts a beta, which is a pre-release and invisible to
stable installs by construction.

So: write the changelog entry in the same pull request as the change. Write it for
the person using the app, not for the person reviewing the diff.

## What is not done yet

Tracked in the planning repo rather than here, because a checklist in a README
rots the moment someone forgets to tick it. The short version: GNOME's native
right-click menu, macOS Finder integration (it needs code signing), Linux
packaging, and a two-machine test of broadcast and resume on real hardware.
