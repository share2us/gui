# Installers

## Component model

One installer, user picks what to install:

- **Share2Us app (GUI)** — the desktop app + Explorer right-click `s2u → Share`.
- **s2u CLI** — the command-line tool, added to `PATH`.
- **Both** (default) — recommended.

The two are **not code-coupled**: the GUI embeds `cli-core` and never shells out
to the CLI. Bundling is a convenience. The real synergy is a **shared login** —
both use the same `cli-core` credential store, so signing in from either signs in
the other.

## Windows (`windows/share2us.iss`, Inno Setup 6)

Per-user install (no admin/UAC): the GUI registers its right-click integration in
`HKCU`; the CLI is appended to the user `PATH`.

```powershell
# 1) gather the two binaries built against the SAME cli-core version
mkdir dist
copy path\to\share2us-gui.exe dist\    # from this repo: wails build
copy path\to\s2u.exe          dist\    # from the share2us/cli repo

# 2) compile the installer (Inno Setup 6 provides ISCC.exe)
iscc /DDistDir=dist /DAppVersion=0.1.0 windows\share2us.iss
# -> Output\Share2Us-Setup-0.1.0.exe
```

Notes:
- The GUI's shell integration is (un)registered via `share2us-gui.exe
  --install-shell` / `--uninstall-shell`, driven from the installer's `[Run]` /
  `[UninstallRun]`.
- PATH is appended only if not already present, and stripped on uninstall.

## Linux / macOS (planned)

- **Linux** — the app's own `--install-shell` already writes the file-manager
  integration (KDE ServiceMenu, Nemo, "Open With"); packaging (`.deb` / AppImage)
  with the same GUI/CLI/both component choice is a follow-up.
- **macOS** — deferred (a signed Finder extension needs an Apple Developer ID).
