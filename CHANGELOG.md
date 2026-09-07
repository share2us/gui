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

### Fixed
- **Nearby devices are found even when the network blocks multicast.** The app
  looked for devices only by name announcement (mDNS), which plenty of ordinary
  networks drop — access points with client isolation, a firewall blocking
  inbound UDP 5353, or a device whose own responder holds the port. On those
  networks two laptops on the same Wi-Fi could never see each other. The app now
  also probes the network directly, recognising a device by its certificate, so
  it appears regardless. A device found that way is listed by address, since no
  name was announced, and shows the same verify code.

## [20260907163356] - 2026-09-07

### Fixed
- **Being discoverable is remembered.** It was only held in memory, so every
  launch started with the device invisible however you had left it, and nearby
  sharing simply appeared not to work. Off is still the default.
- **"Receive files sent to this device" at setup now means it.** That box only
  made the app start at login, and an app that is not discoverable receives
  nothing. It now turns on receiving as well.
- The address box under a device no longer says **"or a code"**. It takes an
  address or an `s2u://` link; the six-digit code is for checking that a device
  is really who it says it is, and was never something you could type in there.
  It also has a **Use** button and accepts Enter, so there is a visible way to
  confirm a typed address.
- The **update** button and the **rescan** button are no longer near-identical
  circular arrows. Update carries an install arrow, and both say what they do.
- The app name and logo no longer appear twice — the window title bar already
  carries them, so the account e-mail takes that space and is no longer clipped.
- **Settings** appeared twice. There is one entry point now, in the status strip.

### Changed
- "Auto-receive files at login" is now **"Start Share2Us at login"**, which is
  what it does; being found by other devices is the Discoverable setting above
  it.
- When the Share2Us background service (`s2u daemon`) is running, the desktop
  app now defers background receiving to it instead of starting its own, so the
  two never both download the same incoming file. No effect when the daemon
  isn't installed.

## [20260904062511] - 2026-09-04

### Security
- Trusting a device now shows its **safety number** (five groups of four
  digits derived from the device key) in the confirmation dialog, to compare
  with the other device's own screen before entering the code. The six-digit
  code stays for per-transfer prompts but is short enough for a determined
  attacker to forge a matching device; the safety number is not. Your own
  safety number appears next to the verify code when Discoverable is on.

## [20260903170224] - 2026-09-03

### Security
- Trusting a nearby device now requires verification through your account
  (ADR-034). After you tick "Trust this device" and accept, Share2Us emails a
  6-digit code to your account address (or asks for your authenticator code
  once you enrol one); the device is trusted only after you confirm it. Trusted
  devices are kept on your account, signed by the server and synced to your
  signed-in machines; a hand-edited local copy is ignored, so an automation or
  AI agent cannot trust devices on its own. Devices trusted before this release
  were never verified this way and are **not carried over**: trust them again
  once. Switching a device to "Save automatically" also asks for a code;
  switching back and revoking do not. Requires being signed in.

## [20260903161240] - 2026-09-03

### Changed
- Trusted devices now have a mode. When you tick "Trust this device" on an
  incoming request you choose what happens next: **Ask before each transfer**
  (default: one tap per file, no code to compare) or **Save its files
  automatically**. Devices trusted before this release are treated as "ask".
  Settings → Trusted devices shows the mode per device and lets you change or
  revoke it.

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
