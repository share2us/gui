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
ArchitecturesInstallIn64BitMode=x64compatible
; Admin: the Share-sheet task trusts the bundled cert (LocalMachine) and installs
; the MSIX. Power users may force per-user, but then that task's cert-trust + MSIX
; steps no-op (they need LocalMachine); the app, CLI and right-click still install.
PrivilegesRequired=admin
PrivilegesRequiredOverridesAllowed=dialog commandline
; We modify the user PATH for the CLI component.
ChangesEnvironment=yes

[Types]
Name: "full"; Description: "Everything (app + command-line)"
Name: "custom"; Description: "Custom"; Flags: iscustom

[Components]
Name: "gui"; Description: "Share2Us app — right-click ""s2u -> Share"" in Explorer"; Types: full custom
Name: "cli"; Description: "s2u command-line tool — adds ""s2u"" to your PATH"; Types: full custom

[Tasks]
Name: "sharesheet"; Description: "Add Share2Us to the Windows ""Share"" menu (Snip & Sketch, Photos, etc.) and trust the Share2.us publisher"; Components: gui
Name: "desktopicon"; Description: "Create a desktop shortcut"; Components: gui; Flags: unchecked
Name: "autoreceive"; Description: "Receive files sent to this device (start at login)"; Components: gui; Flags: unchecked

[Files]
Source: "{#DistDir}\{#GuiExe}"; DestDir: "{app}"; Components: gui; Flags: ignoreversion
; skipifsourcedoesntexist: build the installer even if the CLI binary wasn't bundled.
Source: "{#DistDir}\{#CliExe}"; DestDir: "{app}"; Components: cli; Flags: ignoreversion skipifsourcedoesntexist
; Share-sheet task payload: the MSIX, the cert that signed it, and the trust+install helper.
Source: "{#DistDir}\*.msix"; DestDir: "{app}"; Components: gui; Tasks: sharesheet; Flags: ignoreversion skipifsourcedoesntexist
Source: "{#DistDir}\*.cer"; DestDir: "{app}"; Components: gui; Tasks: sharesheet; Flags: ignoreversion skipifsourcedoesntexist
Source: "sharesheet.ps1"; DestDir: "{app}"; Components: gui; Tasks: sharesheet; Flags: ignoreversion

[Icons]
Name: "{group}\Share2Us"; Filename: "{app}\{#GuiExe}"; Components: gui
Name: "{group}\Uninstall Share2Us"; Filename: "{uninstallexe}"
Name: "{userdesktop}\Share2Us"; Filename: "{app}\{#GuiExe}"; Components: gui; Tasks: desktopicon

[Run]
; Register the Explorer right-click integration for the current user.
Filename: "{app}\{#GuiExe}"; Parameters: "--install-shell"; Components: gui; Flags: runhidden
; Trust the bundled cert (LocalMachine) + install the MSIX (Windows Share sheet).
; Runs in the installer's elevated context, so no nested self-elevation is needed.
Filename: "powershell.exe"; Parameters: "-NoProfile -ExecutionPolicy Bypass -File ""{app}\sharesheet.ps1"""; \
  StatusMsg: "Adding Share2Us to the Windows Share menu..."; Components: gui; Tasks: sharesheet; Flags: runhidden waituntilterminated
; Optionally start the background receiver at login.
Filename: "{app}\{#GuiExe}"; Parameters: "--enable-autostart"; Components: gui; Tasks: autoreceive; Flags: runhidden
; Offer to launch the app at the end.
Filename: "{app}\{#GuiExe}"; Description: "Launch Share2Us"; Components: gui; Flags: nowait postinstall skipifsilent

[UninstallRun]
Filename: "{app}\{#GuiExe}"; Parameters: "--uninstall-shell"; Components: gui; Flags: runhidden; RunOnceId: "s2uUnregisterShell"
; Best-effort removal of the Share-sheet MSIX package. No { } scriptblock here:
; Inno parses braces as constants. Get-AppxPackage takes a wildcard name directly.
Filename: "powershell.exe"; Parameters: "-NoProfile -ExecutionPolicy Bypass -Command ""Get-AppxPackage *Share2Us* | Remove-AppxPackage"""; \
  Flags: runhidden; RunOnceId: "s2uRemoveMsix"

[Registry]
; Append the install dir to the user PATH so `s2u` works from any terminal.
Root: HKCU; Subkey: "Environment"; ValueType: expandsz; ValueName: "Path"; \
  ValueData: "{olddata};{app}"; Components: cli; Check: NeedsAddPath('{app}'); \
  Flags: preservestringtype

[Code]
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

{ On uninstall, strip our install dir from the user PATH (best effort). }
procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
var
  OrigPath, AppDir, Needle: string;
  P: Integer;
begin
  if CurUninstallStep <> usUninstall then
    exit;
  if not RegQueryStringValue(HKCU, 'Environment', 'Path', OrigPath) then
    exit;
  AppDir := ExpandConstant('{app}');
  Needle := ';' + Uppercase(OrigPath) + ';';
  P := Pos(';' + Uppercase(AppDir) + ';', Needle);
  if P = 0 then
    exit;
  { Rebuild PATH without the AppDir segment. }
  Delete(OrigPath, P, Length(AppDir) + 1);
  { Trim any accidental leading/trailing ';'. }
  while (Length(OrigPath) > 0) and (OrigPath[1] = ';') do Delete(OrigPath, 1, 1);
  while (Length(OrigPath) > 0) and (OrigPath[Length(OrigPath)] = ';') do Delete(OrigPath, Length(OrigPath), 1);
  RegWriteExpandStringValue(HKCU, 'Environment', 'Path', OrigPath);
end;
