# Windows distribution & the "dangerous/virus" flag

## What happened

Chrome (Google Safe Browsing) + Windows Defender hard-blocked `Setup.exe` as
**"dangerous/virus."**

Root cause was **not** the binary (it isn't packed — clean PE sections). It was a
**behavior**: the optional "Share sheet" task imported a bundled **self-signed
certificate into `Cert:\LocalMachine\Root`** (the Trusted Root CA store) and then
sideloaded an MSIX. *An installer that adds a root CA certificate* is a textbook
malware pattern — it is how malware gets its own payloads trusted / enables MITM —
so Defender and Safe Browsing behavioral analysis block it. With the installer
also being unsigned by a real CA, that produced the hard verdict.

**Fixed** (commit `c1f82ae`): the root-store trust + local MSIX sideload were
removed from `share2us.iss` and `sharesheet.ps1` was deleted. `Setup.exe` now
installs the app + CLI only. The Share-sheet MSIX moves to the Microsoft Store.

That removes the malware *behavior*. `Setup.exe` is still unsigned by a CA, so
SmartScreen may still show a soft **"unknown publisher"** prompt on run (click
**More info → Run anyway**), but the hard "virus" block should be gone. The
durable fix for *all* browser/SmartScreen friction is a trusted distribution
channel — below.

## Why not just buy a code-signing cert

Verified 2026-07 (owner is a Pakistan-based individual, pre-revenue):

- **Azure Trusted / Artifact Signing** (~$10/mo, the cheap modern route) —
  individual developers are limited to **USA & Canada** (orgs: US/Canada/EU/UK).
  Not available here. It also doesn't grant *instant* SmartScreen trust.
- **OV code-signing cert** (~$200/yr) — establishes a verified publisher but
  SmartScreen still warms up over download volume; needs an HSM/cloud key and
  payment from PK.
- **EV cert** (~$300–600/yr) — instant SmartScreen trust, but needs a hardware
  token **and a registered business entity**.

So the chosen path is **distribution channels that carry their own trust**:
Microsoft Store (primary) + winget (secondary). Both sidestep the Chrome download
block entirely.

## Path A — Microsoft Store (primary, cleanest)

The Store signs the MSIX with a **Microsoft-trusted cert**, so there's no
self-signed anything and Store apps are never Safe-Browsing/SmartScreen-flagged.

**Owner-side (one-time):**
1. Create a **Microsoft Partner Center** individual developer account — **$19
   one-time** (the only cost; watch the PK-payment caveat, but it's one-time).
2. **Reserve the app name** "Share2Us" (Apps and Games → New product → MSIX/PWA).
3. Complete identity verification.
4. From the product's **Product Identity** page, copy the assigned
   **Package/Identity Name**, **Publisher** (`CN=...`), and **Publisher display
   name**.

**Done:** account created + app reserved (**Store ID `9NFR4MMM7RJ8`**,
https://apps.microsoft.com/detail/9NFR4MMM7RJ8). Identity wired into
`installer/msix/AppxManifest.xml` (`Name=Share2Us.Share2us`,
`Publisher=CN=8F6CFF38-67FC-45C0-9A09-AF6CCD6B9EC9`). The release workflow now
builds an **unsigned** `Share2Us-<ver>.msix` (the Store re-signs it) and publishes
it as a release asset. Identity values live in
`credentials/microsoft_store.conf` (git-ignored).

**Remaining (owner, first submission is manual in Partner Center):**
- Download `Share2Us-<latest>.msix` from the newest GitHub release.
- Partner Center → the reserved product → **Packages** → upload the `.msix`.
- Fill the listing (description, screenshots, category), **age rating**
  (IARC questionnaire), privacy policy URL, and the **runFullTrust
  justification** ("Desktop application packaged as MSIX").
- Submit for certification. (The Store Submission API can automate later.)

**Still pending regardless (needs Windows):** the Share Target *file hand-off*
(receiving the shared file when the user picks S2u) — see
`installer/msix/README.md` and `internal/sharetarget/sharetarget_windows.go`. The
app appears in the Share sheet today but doesn't yet consume the shared file.

## Path B — winget (secondary, points at the GitHub release)

winget installs via PowerShell, so Chrome's download block never applies. winget's
validation runs the installer in a Defender sandbox — which is why the root-cert
fix above was a prerequisite (it would have failed validation before).

**Prep / submission (I can do on your go-ahead — it's a public PR to
`microsoft/winget-pkgs`, so confirm first):**
- Our Inno installer already supports the silent switches winget needs
  (`/VERYSILENT /SUPPRESSMSGBOXES /NORESTART /SP-`), and returns standard exit
  codes.
- Easiest tooling: `wingetcreate`:
  ```powershell
  wingetcreate new https://github.com/share2us/gui/releases/download/<tag>/Share2Us-Setup-<tag>.exe
  # PackageIdentifier: Share2us.Share2Us ; InstallerType: inno ; Scope: machine
  wingetcreate submit --token <PAT>   # opens the PR to microsoft/winget-pkgs
  ```
- Because our version is a UTC timestamp, this is per-release; a `wingetcreate
  update` CI step (needs a PAT secret) can auto-PR new versions on each release.

## Interim free mitigations (until Store/winget land)

- **Report the false positive** so the blocked URL clears faster:
  Google Safe Browsing — https://safebrowsing.google.com/safebrowsing/report_error/
  · Microsoft Defender — https://www.microsoft.com/wdsi/filesubmission
- **Point users at the `.zip`** (`share2us-gui_windows_amd64.zip`) rather than the
  bare `Setup.exe` — Chrome rarely hard-blocks archives. (Portable app, no
  installer; the right-click/Store extras don't apply.)
- **Install instructions:** in Chrome, if warned, use the download's **⋮ → Keep**;
  on run, **More info → Run anyway** for the SmartScreen prompt.
