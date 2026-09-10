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

### Security
- **An encrypted file now gets its own key.** Every encrypted share used its key
  directly and told the pieces of the file apart with four random bytes. That was
  safe only for as long as one key was never used to encrypt twice, which nothing
  enforced and nothing showed. Two files encrypted under one key that drew the
  same four bytes would have given away both of them. Each file now derives a key
  of its own, so a key can encrypt as many files as it likes.
- Files encrypted by earlier versions still open. **Files encrypted by this
  version do not open in an earlier one**, which reports the format as
  unsupported. Update both ends.

## [20260910065215] - 2026-09-10

### Added
- **See your devices in Settings.** The machines signed in to your account, and
  whether each one is ready to receive a file or still needs you to sign in on
  it. Previously you could only find this out by starting a share.
- **Send to your own devices over the internet, not just across the network.**
  "My device, anywhere" sits beside the nearby options in the send flow: pick one
  of the machines signed in to your account and the file goes there, encrypted
  end to end, from anywhere. The app could already do this — nothing in it
  reached the feature, so it only existed in the command line.
- **Files you are sent no longer appear in Downloads without you choosing.**
  Files sent to this device now wait until you say where they go, or go straight
  to the folder you picked — the same way files received over the local network
  already behaved. Two other things came with it: a second file of the same name
  no longer overwrites the first, and a save folder that has gone away (an
  unplugged drive, a revoked permission) leaves the file waiting instead of
  losing it.
- **"Only me": upload a file without sharing it with anyone.** A third option on
  the "Create a link" tab, beside "Anyone with the link" and "Only these people".
  The file goes into your account and is shared with nobody — the link only opens
  for you. Sign in on another of your machines, paste the link, and you have the
  file. Until now the app could only make a link anyone could open or one gated
  on an email address, so there was no way to simply put a file somewhere you
  could reach it later.

### Fixed
- **"Get beta builds" never found a build.** The check asked GitHub for the last
  thirty releases and refused to read a reply that large, so every beta check
  ended in an error the app reported as "you are up to date". Anyone who opted in
  has been sitting on whatever build they installed by hand. It now asks for the
  newest few and reads the whole reply.
- **Buttons that are waiting on something now say so.** Several — installing an
  update, signing out, choosing a folder, answering an incoming transfer — sat
  there looking unclicked while they worked. They now show the same working
  indicator the rest of the app uses.
- **Settings no longer closes itself.** Changing a setting inside it (toggling
  "Discoverable on local network", for instance) collapsed the whole panel.

### Security
- **The app now acts only on files you actually chose.** Sharing, sending over
  the network, broadcasting and picking a receive folder each took whatever path
  they were handed. Nothing could reach them today — the pages are the app's own
  and external links open in your browser — but if anything ever did, a single
  call would have been enough to upload a file you never picked. A path is now
  usable only when it came from the file picker, a drag onto the window, the
  folder picker, or the "Share" menu in Explorer.
- **Peer names from the network can no longer write into the app's own text.** A
  device chooses its display name, and that name was shown untouched. It is now
  stripped of anything that can reorder or hide what you are reading.
- **Trusted devices are verified against keys built into the app**, and against
  the account you are signed in to. The list used to be checked with a key kept
  in the same file, so anything able to write that file could add itself as a
  trusted device and have its files saved without asking.
- **Files kept for an in-flight transfer are cleared when you sign out**, and
  removed from disk when they expire rather than merely ignored.
- Dependency advisories: `x/net` v0.56.0 (a panic a remote peer could trigger)
  and `x/text` v0.39.0.

## [20260908130000] - 2026-09-08

### Added
- **Nearby devices have names again.** A device found by probing the network used
  to show as a bare address, because only the older name announcement carried a
  name and that announcement is blocked on a lot of networks: Windows machines set
  to Public, anything on a different subnet, and every device reached over
  Tailscale. Each device now publishes its own name, signed with its identity, so
  nobody else on the network can claim it.
- **You can give a device your own name.** Three laptops called DESKTOP-4F2K9A is
  not a security problem and is still an unusable list. Press the pencil beside a
  device to name it. Only you see the name, it does not rename the other device,
  and it does not trust it. The name sticks to the device itself, so it survives
  the device changing address, moving network, or restarting.
- **This device's address is now always in the bar at the bottom**, and in the
  window title. Before it was only shown while you were discoverable, which is the
  opposite of when you need it: you go looking for your address while working out
  why the other laptop cannot see you.
- **You can cancel a file you did not want.** Every incoming file now has an X
  beside it. A file you have not saved yet asks first, because deleting it cannot
  be undone; one already saved to your folder just leaves the list, and the file
  stays where you put it.

### Changed
- **Accepting a file now asks where to put it straight away.** Before, you
  approved the transfer and then nothing appeared to happen, because the file was
  waiting for you to find a save button. Cancelling still leaves it waiting, and
  if you have chosen a folder to always save to, nothing asks at all.

### Fixed
- **Buttons now show they were pressed.** Every button dips slightly on click,
  and one that starts something slow, choosing files, scanning the network,
  sending, shows a moving bar along its edge until the work is done. Pressing
  rescan while a scan was already running used to do nothing at all, which looked
  the same as the app hanging; it now waits for the scan in progress.
- **The address shown for this device was sometimes the wrong one.** On a machine
  with WSL, Hyper-V, Docker or a virtual machine installed, the app could show a
  private address belonging to one of those, such as 172.21.208.1, while the other
  device saw the real one, such as 192.168.10.218. The real network adapter now
  wins.
- **Tailscale devices were never found.** Two separate reasons, either one enough
  on its own: the app could not locate the Tailscale program on Windows or macOS,
  and its routine check for nearby devices skipped the Tailscale network
  altogether, so a device could only appear on a manual refresh at best.
- **A device restarting no longer looks like an impostor.** The app was
  remembering devices by a value that changes every time they restart, so an
  ordinary reboot came back as a warning that the device was not the one you sent
  to last time. It now remembers the device's real identity, which does not change,
  and the warning is kept for what it was meant for.
- **The code shown for a device now matches the code that device shows.** The
  6-digit code beside a device in the list and the code in that same device's
  transfer prompt were two different numbers.

## [20260908111040] - 2026-09-08

### Changed
- **The download prompt now says whether the device is who it claims.** A name on
  the network can be claimed by anything, so before downloading you see the code
  to check against that device's screen. If a device you have downloaded from
  before turns up with a different identity, the prompt says so, shows both codes,
  and stops making Download the inviting button.

## [20260908103919] - 2026-09-08

### Added
- **The app now notices if a nearby device changes identity.** A device's name on
  the network can be claimed by anything, so the app remembers which device
  answered to a name last time. The first time you send somewhere it shows the
  code once so you can check it matches that device's screen; after that it stays
  quiet, and speaks up only if the name later shows a different identity, which is
  what impersonation looks like.

### Fixed
- Re-issues the previous release, which was published without the Windows
  installer because the build stopped partway. Nothing was wrong with the app
  itself; the release was simply incomplete. If you installed from it, this one
  is complete.

## [20260908063848] - 2026-09-08

### Fixed
- **The app and the bundled command-line tool are now actually signed.** A fault
  in the release pipeline meant only the installer carried a signature; the two
  programs it installs never did. Windows treats unsigned programs with more
  suspicion, which is exactly the problem the command-line tool has been hitting.

### Added
- **The command-line tool ships inside the Microsoft Store package.** Installed
  from the Store it is Store-signed, which is what Windows Smart App Control
  requires, and `s2u` works from any terminal. Updates for that copy come from
  the Store rather than the app updating itself.

## [20260908055209] - 2026-09-08

### Added
- **The licence now installs with the app.** LICENSE.txt and
  THIRD-PARTY-NOTICES.txt land in the install folder. Share2Us is free software
  under the GPLv3; it was not shipping the text that says so. Deliberately not a
  click-through page during setup: the GPL needs no acceptance to use the
  software, and presenting it as an agreement would misrepresent it.

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
