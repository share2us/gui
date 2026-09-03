# Changelog

All notable user-facing changes to the Share2Us desktop app.

The release workflow reads the `[Unreleased]` section below into the GitHub
release notes, and **refuses to cut a stable release while it is empty** — so a
build cannot reach users without saying what changed. On release, move the
section under a new version heading. HTML comments do not count as content, so
a section holding only a note still blocks a stable release.

Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
Versions are UTC build timestamps (`20260902114433`), not semver.

## [Unreleased]

<!-- Add user-facing changes here as they merge. A stable release refuses to
     ship while this section is empty (HTML comments do not count). -->

## [20260903141453] - 2026-09-03

### Added
- Settings gains "Get beta builds": the app then offers pre-release builds
  before they reach everyone, and shows "Beta update available" when the offered
  build is one. The choice is shared with the `s2u` command line on this
  machine (`s2u update --channel beta`), so both follow one channel. Off by
  default; stable installs cannot see a beta. Store installs are unaffected.

## [20260903095016] - 2026-09-03

### Security
- Windows updates from the direct download are now pinned to the publisher's
  signing certificate: an update signed by a different certificate is refused
  even if it also names Share2.us. Requires a stable signing certificate in CI;
  builds made without one keep verifying by checksum alone. The Microsoft Store
  package is unaffected — the Store signs it, and Store builds have no updater.

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
