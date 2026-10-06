; Share2Us Windows installer (Inno Setup).
;
; Component-selection install: the user picks the GUI app, the s2u CLI, or both.
; Both share the same login (cli-core credential store), so signing in from either
; also signs in the other. The GUI registers its Explorer right-click integration
; in HKCU and the CLI is added to the user PATH.
;
; Runs as admin so the optional "Windows Share sheet" task can trust the bundled
; self-signed cert (LocalMachine stores -> publisher shows as "Share2.us" instead
; of "unknown") and install the bundled MSIX (registers S2u in the Windows Share
; sheet / Snip & Sketch "Share"). UAC keeps HKCU pointed at the invoking user, so
; the right-click verb + PATH still land in that user's hive.
;
; Build (on Windows, with Inno Setup 6):
;   1) put the built artifacts in a folder, e.g. dist\share2us-gui.exe, dist\s2u.exe,
;      dist\Share2Us.msix and dist\Share2Us.cer (the same cert that signed the msix)
;   2) iscc /DDistDir=dist /DAppVersion=0.1.0 installer\windows\share2us.iss
; The CLI binary comes from the share2us/cli repo built against the SAME cli-core
; version as the GUI, so the pair stays in lockstep.

#define AppName "Share2Us"
#ifndef AppVersion
  #define AppVersion "0.0.0"
#endif
#define AppPublisher "Share2.us"
#define AppURL "https://share2.us"
#define GuiExe "share2us-gui.exe"
#define CliExe "s2u.exe"
#ifndef DistDir
  #define DistDir "dist"
#endif
; The CLI lives per user, in the same folder install.ps1 uses, not under
; Program Files. There `s2u update` can replace it without admin rights, and a
; CLI installed with `irm https://share2.us/install.ps1 | iex` is the same copy
; rather than a second one. Before 2026-09-30 it went to {app}: that copy could
; never update itself, and it sat first on the user PATH, so it shadowed every
; newer CLI (owner, 2026-09-30: the VM ran a two-week-old s2u).
#define CliDir "{localappdata}\Share2Us\bin"

[Setup]
AppId={{A7F3C1E2-5B84-4D93-9E17-2C6A8F0B4D51}
AppName={#AppName}
AppVersion={#AppVersion}
AppPublisher={#AppPublisher}
AppPublisherURL={#AppURL}
; File properties of Setup.exe (Details tab) + the installer's version resource.
VersionInfoCompany={#AppPublisher}
VersionInfoProductName={#AppName}
VersionInfoDescription=Share2Us Setup
DefaultDirName={autopf}\Share2Us
DefaultGroupName=Share2Us
DisableProgramGroupPage=yes
OutputBaseFilename=Share2Us-Setup-{#AppVersion}
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
; The installer's OWN icon. Without this Setup.exe ships with Inno Setup's
; default, which is the very first thing a user sees of Share2Us and the moment
; they decide whether the download is what they think it is. The app's shortcuts
; already get the icon for free (Windows reads it out of the exe, where Wails
; embeds it); the installer was the one surface with nothing.
SetupIconFile=..\..\build\windows\icon.ico
ArchitecturesInstallIn64BitMode=x64compatible
; Admin: the Share-sheet task trusts the bundled cert (LocalMachine) and installs
; the MSIX. Power users may force per-user, but then that task's cert-trust + MSIX
; steps no-op (they need LocalMachine); the app, CLI and right-click still install.
PrivilegesRequired=admin
PrivilegesRequiredOverridesAllowed=dialog commandline
; We modify the user PATH for the CLI component.
ChangesEnvironment=yes
; Add/Remove Programs shows this next to the entry. It defaults to the
; uninstaller's icon, which is not the app's.
UninstallDisplayIcon={app}\{#GuiExe}
; PrepareToInstall (see [Code]) stops any running Share2Us processes before the
; copy, so the Restart Manager finds nothing holding s2u.exe / share2us.exe and
; never falls back to demanding a Windows restart. Do not let it try to relaunch
; what it closed: the headless receiver cannot be restarted that way, and the app
; is relaunched by the postinstall step / the receiver by autostart instead.
RestartApplications=no

[Types]
Name: "full"; Description: "Everything (app + command-line)"
Name: "custom"; Description: "Custom"; Flags: iscustom

[Components]
Name: "gui"; Description: "Share2Us app — right-click ""s2u -> Share"" in Explorer"; Types: full custom
Name: "cli"; Description: "s2u command-line tool — adds ""s2u"" to your PATH"; Types: full custom

; NOTE: the Windows "Share" sheet integration (the MSIX) is intentionally NOT
; installed here. It used to be sideloaded by trusting a bundled self-signed cert
; into LocalMachine\Root — but "an installer adds a root CA certificate" is a
; textbook malware behavior that Defender + Safe Browsing hard-block ("dangerous/
; virus"). The Share-sheet MSIX now ships through the Microsoft Store, which signs
; it with a trusted cert (no root-store tampering, no self-signed anything).
[Tasks]
Name: "desktopicon"; Description: "Create a desktop shortcut"; Components: gui; Flags: unchecked
Name: "autoreceive"; Description: "Receive files sent to this device (start at login)"; Components: gui; Flags: unchecked

[Files]
Source: "{#DistDir}\{#GuiExe}"; DestDir: "{app}"; Components: gui; Flags: ignoreversion
; skipifsourcedoesntexist: build the installer even if the CLI binary wasn't bundled.
; Both names, as install.ps1 ships them: s2u.exe and share2us.exe are the same binary.
Source: "{#DistDir}\{#CliExe}"; DestDir: "{#CliDir}"; Components: cli; Flags: ignoreversion skipifsourcedoesntexist
Source: "{#DistDir}\{#CliExe}"; DestDir: "{#CliDir}"; DestName: "share2us.exe"; Components: cli; Flags: ignoreversion skipifsourcedoesntexist
; The licence ships NEXT TO the app, not as a click-through page. Share2Us is
; GPL-3.0-only, and the GPL needs no acceptance to USE the software — presenting
; it as an EULA you must agree to before installing misrepresents what it is.
; MUI_PAGE_LICENSE stays off for that reason; this is the honest alternative.
; Named .txt so it opens on a double-click; Windows will not open an
; extensionless file. Paths are relative to THIS file's directory (no SourceDir
; is set), so ..\..\ is the repo root.
Source: "..\..\LICENSE"; DestDir: "{app}"; DestName: "LICENSE.txt"; Flags: ignoreversion
Source: "..\..\THIRD-PARTY-NOTICES.md"; DestDir: "{app}"; DestName: "THIRD-PARTY-NOTICES.txt"; Flags: ignoreversion

[InstallDelete]
; The CLI copy an older installer put under Program Files (see CliDir).
Type: files; Name: "{app}\{#CliExe}"

[Icons]
Name: "{group}\Share2Us"; Filename: "{app}\{#GuiExe}"; Components: gui
Name: "{group}\Uninstall Share2Us"; Filename: "{uninstallexe}"
Name: "{userdesktop}\Share2Us"; Filename: "{app}\{#GuiExe}"; Components: gui; Tasks: desktopicon

[Run]
; Allow the app inbound on the local network so device discovery (mDNS multicast,
; UDP 5353) and incoming transfers work. Without an explicit rule Windows Firewall
; blocks inbound by default, and mDNS multicast RECEIVE often never triggers the
; interactive "allow" prompt — so "nearby devices" stays empty. Program-scoped, so
; it covers the TCP transfer port and UDP mDNS without naming either.
;
; ALL profiles, including public. Firewall rules are per-profile, so the earlier
; private+domain rule simply did not apply on a network Windows had classified as
; Public — and Windows classifies plenty of ordinary home networks that way,
; including behind an Apple router. The result was two laptops on the same Wi-Fi
; that could never see each other, with nothing on screen to say why (owner,
; 2026-09-07).
;
; The exposure this accepts is a listening port and an advertisement, not open
; file acceptance: every inbound transfer is approved by the user, an untrusted
; sender is shown with a verify code to compare, and trusting a device needs a
; second factor. The installer is elevated, so netsh can add it.
; Remove any rule from a previous install first: netsh ADDS a second rule with the
; same name rather than replacing one, so upgrades would otherwise stack copies —
; including the old private-only rule this replaces.
Filename: "{sys}\netsh.exe"; Parameters: "advfirewall firewall delete rule name=""Share2Us local sharing"""; Components: gui; Flags: runhidden
Filename: "{sys}\netsh.exe"; Parameters: "advfirewall firewall add rule name=""Share2Us local sharing"" dir=in action=allow program=""{app}\{#GuiExe}"" enable=yes profile=any"; Components: gui; Flags: runhidden; StatusMsg: "Allowing local-network sharing through Windows Firewall..."
; Register the Explorer right-click integration for the current user.
Filename: "{app}\{#GuiExe}"; Parameters: "--install-shell"; Components: gui; Flags: runhidden
; Optionally start the background receiver at login.
Filename: "{app}\{#GuiExe}"; Parameters: "--enable-autostart"; Components: gui; Tasks: autoreceive; Flags: runhidden
; Offer to launch the app at the end.
Filename: "{app}\{#GuiExe}"; Description: "Launch Share2Us"; Components: gui; Flags: nowait postinstall skipifsilent

[UninstallRun]
Filename: "{sys}\netsh.exe"; Parameters: "advfirewall firewall delete rule name=""Share2Us local sharing"""; Flags: runhidden; RunOnceId: "s2uDelFwRule"
Filename: "{app}\{#GuiExe}"; Parameters: "--uninstall-shell"; Components: gui; Flags: runhidden; RunOnceId: "s2uUnregisterShell"

[Registry]
; Append the CLI dir to the user PATH so `s2u` works from any terminal (a new
; one: a window that was already open keeps its old PATH).
Root: HKCU; Subkey: "Environment"; ValueType: expandsz; ValueName: "Path"; \
  ValueData: "{olddata};{#CliDir}"; Components: cli; Check: NeedsAddPath('{#CliDir}'); \
  Flags: preservestringtype

[Code]
{ Stop any running Share2Us process before the install overwrites its files.

  The old blocker: the background receiver is `s2u daemon run`, a HEADLESS
  console process with no tray icon and no window. Inno's default Restart Manager
  can close the GUI window, but it cannot shut a headless console process down,
  so it left s2u.exe / share2us.exe locked and Setup demanded a Windows restart
  before it would continue -- with nothing on screen to say what "s2u" even was
  (owner, 2026-10-06: "says s2u is running but there's no s2u in the system tray,
  won't proceed until I restart Windows").

  We stop it ourselves, here, before any file is touched. Graceful first: the
  installed CLI asks the daemon to stop over its control socket, which releases
  any bound agent session cleanly and also catches a hand-started `daemon run`.
  Then we force-kill whatever is left by image name. All best effort -- a failure
  here must never block the install, so return codes are ignored. }
procedure StopRunningShare2Us;
var
  ResultCode: Integer;
  CliPath: string;
begin
  CliPath := ExpandConstant('{#CliDir}\{#CliExe}');
  if not FileExists(CliPath) then
    CliPath := ExpandConstant('{#CliDir}\share2us.exe');
  if FileExists(CliPath) then
    Exec(CliPath, 'daemon stop', '', SW_HIDE, ewWaitUntilTerminated, ResultCode);
  { Force-kill anything still up: the receiver/CLI (s2u.exe and its share2us.exe
    twin, from any path) and the GUI (share2us-gui.exe). /T takes child processes
    too. Setup.exe is named none of these, so it is never a target. }
  Exec(ExpandConstant('{sys}\taskkill.exe'), '/F /T /IM s2u.exe', '', SW_HIDE, ewWaitUntilTerminated, ResultCode);
  Exec(ExpandConstant('{sys}\taskkill.exe'), '/F /T /IM share2us.exe', '', SW_HIDE, ewWaitUntilTerminated, ResultCode);
  Exec(ExpandConstant('{sys}\taskkill.exe'), '/F /T /IM {#GuiExe}', '', SW_HIDE, ewWaitUntilTerminated, ResultCode);
  { Let Windows release the file handles before the copy begins. }
  Sleep(800);
end;

{ The last event before the install proper -- Setup checks for files in use after
  this, so by then nothing of ours is holding them. }
function PrepareToInstall(var NeedsRestart: Boolean): String;
begin
  StopRunningShare2Us;
  Result := '';
end;

// True when the install dir is not already on the user PATH (avoids duplicates).
function NeedsAddPath(Param: string): Boolean;
var
  OrigPath: string;
begin
  if not RegQueryStringValue(HKCU, 'Environment', 'Path', OrigPath) then
  begin
    Result := True;
    exit;
  end;
  Result := Pos(';' + Uppercase(ExpandConstant(Param)) + ';', ';' + Uppercase(OrigPath) + ';') = 0;
end;

{ Remove one directory from the user PATH, if it is there (best effort). }
procedure RemoveFromUserPath(Dir: string);
var
  OrigPath, Needle: string;
  P: Integer;
begin
  if not RegQueryStringValue(HKCU, 'Environment', 'Path', OrigPath) then
    exit;
  Needle := ';' + Uppercase(OrigPath) + ';';
  P := Pos(';' + Uppercase(Dir) + ';', Needle);
  if P = 0 then
    exit;
  { Rebuild PATH without the Dir segment. }
  Delete(OrigPath, P, Length(Dir) + 1);
  { Trim any accidental leading/trailing ';'. }
  while (Length(OrigPath) > 0) and (OrigPath[1] = ';') do Delete(OrigPath, 1, 1);
  while (Length(OrigPath) > 0) and (OrigPath[Length(OrigPath)] = ';') do Delete(OrigPath, Length(OrigPath), 1);
  RegWriteExpandStringValue(HKCU, 'Environment', 'Path', OrigPath);
end;

{ After installing: drop the Program Files entry an older installer added for
  the CLI, so it cannot shadow the per-user copy. }
procedure CurStepChanged(CurStep: TSetupStep);
begin
  if CurStep = ssPostInstall then
    RemoveFromUserPath(ExpandConstant('{app}'));
end;

{ On uninstall, strip our directories from the user PATH. }
procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
begin
  if CurUninstallStep <> usUninstall then
    exit;
  { Same reason as install: a running headless receiver would otherwise lock
    s2u.exe / share2us.exe and leave files behind. usUninstall fires before any
    file is removed, so stopping here clears the hold first. }
  StopRunningShare2Us;
  RemoveFromUserPath(ExpandConstant('{#CliDir}'));
  RemoveFromUserPath(ExpandConstant('{app}'));
end;
