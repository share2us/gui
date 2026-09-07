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

<!-- Add user-facing changes here as they merge. A merge to main with this
     section empty cuts NO release; write a line here and the merge ships. -->

## [20260907200321] - 2026-09-07

### Fixed
- **Signing in no longer moves the window while you are reading the code off it.**
  The message went through three different heights in one flow.

## [20260907195114] - 2026-09-08

### Fixed
- **The window no longer jumps a moment after it opens.** Nearby devices, incoming
  files and recent activity all arrive a beat after the window appears, and the
  layout used to settle around them. The space is now held from the first frame.
- **Settings stops resizing while you are in it.** "Clear activity log", "Ask each
  time" and the sign-out row appeared only once they had something to act on, so
  the panel changed height underneath you. They are always there, and dimmed when
  they do not apply.

## [20260907194514] - 2026-09-08

### Fixed
- **The keyboard can reach everything again.** Choosing where a file goes was
  drawn as plain boxes with no keyboard behaviour at all, so that choice could
  only be made with a mouse. The destination cards and the broadcast access modes
  are now focusable and answer to Enter and Space.
- **Text boxes and dropdowns show a focus outline.** A style rule was cancelling
  it on every one of them, so tabbing through the window gave no clue where you
  were. Checked across every stop in the tab order.
- **An update no longer announces itself twice, or moves the window while you are
  reading it.** The bar that pushed the whole app down when a check came back is
  gone; the dot on the update icon says one is ready, and clicking it opens a
  panel with the version and Install.
- **The line under the main button no longer resizes the window** as files are
  added or a device is chosen, and the clipboard suggestion no longer nudges the
  drop area every time you switch back to the app.
- **Warning messages follow the light theme.** Their colours were fixed to the
  dark palette, so in light mode they came out wrong.
- Two back buttons and the file-count now say what they are to a screen reader.

## [20260907192031] - 2026-09-08

### Fixed
- **A received file no longer overwrites one already in your folder.** With a save
  folder remembered, an arrival with a name already there was written straight
  over it — the earlier file was gone, with no prompt and no record. It now lands
  as "report (1).pdf".
- **Two files with the same name can both arrive.** While one was waiting to be
  saved, a second copy of the same name failed the whole transfer and told the
  sender to re-run with a command-line flag this app does not have.
- **Files saved to your remembered folder still appear under Incoming**, so a
  single file can be sent somewhere else without changing the setting and back.
  Nothing there is ever deleted: a file already saved simply stops being listed.
- **"Only these people" now asks who.** Choosing it gave you the same options as a
  public link, and the share was created with an empty recipient list — the
  restriction could not be set at all.
- **Broadcast no longer claims more files than it sends.** The button offered to
  broadcast every selected file and sent only the first. Broadcast handles one
  file at a time and now says so.
- **Settings can be closed again.** The link at the bottom only ever opened it.
- **An incoming file can be saved by clicking its row**, not only the small icon.

### Added
- **Links to share2.us and the source code** beside the version at the bottom of
  the window. Share2Us is free software (GPL-3.0-only); the matching
  `s2u version` change prints both.

## [20260907172915] - 2026-09-07

### Fixed
- **Windows Firewall no longer blocks sharing on a "Public" network.** The
  installer's firewall rule only covered Private and Domain networks, and Windows
  classifies plenty of ordinary home networks as Public — including behind an
  Apple router. On those, inbound discovery and transfers were dropped with
  nothing on screen to say why, so two laptops on the same Wi-Fi could never see
  each other. The rule now covers every profile, and reinstalling replaces the old
  rule instead of stacking a second one.
- **The app now says so when Windows has the network set to Public**, with a
  button to open the Windows network settings. It only says it when Windows
  actually reported Public: an uncertain answer says nothing, rather than telling
  you something about your network that may not be true.
- **No more console window flashing on Windows.** Looking for nearby devices ran
  the `tailscale` command each time, and a desktop app starting a console program
  opens a console. It now runs hidden, and is not run at all when Tailscale is
  not installed.
- **The routine check no longer probes every address on the network.** A full
  sweep happens when you press refresh, and unattended at most once every ten
  minutes; the check in between re-probes only devices already seen. A desktop
  app connecting to every address once a minute is the signature of a port
  scanner, and antivirus software is right to treat it as one.
- **The arrow beside a nearby device does something.** From the home screen no
  files were chosen yet, so it sent an empty list — nothing happened, and an
  empty send counted as a success, so nothing was reported either. It now asks
  which files to send and opens the send flow with that device chosen.
- **This device's own address is shown** in the status strip beside its verify
  code, with every address of the machine in the tooltip, so matching your device
  in someone else's list no longer means going to look it up in Windows.
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
