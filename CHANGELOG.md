# Changelog

All notable user-facing changes to the Share2Us desktop app.

The release workflow reads the `[Unreleased]` section below into the GitHub
release notes, and **refuses to cut a stable release while it is empty** — so a
build cannot reach users without saying what changed. On release, move the
section under a new version heading. HTML comments do not count as content, so
a section holding only a note still blocks a stable release.

`[Unreleased]` is genuinely empty right now, and that is correct: the last
release (`v20260801223900`, 2026-08-01) already contains everything on `main`
except the release-workflow change itself, which is not user-facing. The
broadcast / home-feed redesign, transfer resume and the LAN security pass all
shipped before that tag — they were released without written notes, which is the
gap this file closes going forward, not a backlog of unreleased work.

Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
Versions are UTC build timestamps (`20260902114433`), not semver.

## [Unreleased]

### Fixed
- The Windows in-app updater could never install an update. It required a
  `Valid` Authenticode status, but the installer stopped importing the signing
  certificate (removed because adding a root CA is what made Defender flag the
  installer as malware), and the release workflow mints a new self-signed
  certificate on every run — so the signature chains to nothing and always reads
  `UnknownError`. Every update offered to a Windows user was refused at that
  gate. Integrity is now verified against a SHA-256 published alongside the
  installer, which the release did not previously include.

## Earlier

Releases before this changelog existed. See the GitHub releases list for the
build history.
