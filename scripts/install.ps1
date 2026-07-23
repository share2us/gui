# Share2Us GUI installer for Windows (PowerShell).
#
#   irm https://raw.githubusercontent.com/share2us/gui/main/scripts/install.ps1 | iex
#
# Always downloads the latest release build for this OS/arch from GitHub Releases
# (the hosted mirror on share2.us is a fallback), verifies its .sha256 sidecar,
# installs it, and registers the Explorer right-click "s2u -> Share" integration.

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
[Net.ServicePointManager]::SecurityProtocol =
  [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12

$Repo       = if ($env:SHARE2US_GUI_REPO)       { $env:SHARE2US_GUI_REPO }       else { 'share2us/gui' }
$Version    = if ($env:SHARE2US_GUI_VERSION)    { $env:SHARE2US_GUI_VERSION }    else { 'latest' }
$BaseUrl    = if ($env:SHARE2US_GUI_BASE_URL)   { $env:SHARE2US_GUI_BASE_URL }   else { 'https://share2.us' }
$InstallDir = if ($env:SHARE2US_GUI_INSTALL_DIR){ $env:SHARE2US_GUI_INSTALL_DIR }else { Join-Path $env:LOCALAPPDATA 'Share2Us' }
$BinaryName = 'share2us-gui.exe'

function Fail($msg) { Write-Error "share2us-gui install: $msg"; exit 1 }
function Log($msg)  { Write-Host $msg }

function Get-Arch {
  switch ($env:PROCESSOR_ARCHITECTURE) {
    'AMD64' { 'amd64' }
    'ARM64' { 'arm64' }
    default { Fail "unsupported architecture: $($env:PROCESSOR_ARCHITECTURE)" }
  }
}

function Download($url, $dest) {
  Invoke-WebRequest -Uri $url -OutFile $dest -UseBasicParsing -Headers @{ 'User-Agent' = 'share2us-gui-installer' }
}

function Download-Verified($dest, $sidecar, [string[]]$urls) {
  foreach ($url in $urls) {
    Log "Trying $url"
    try {
      Download $url $dest
      Download "$url.sha256" $sidecar
    } catch { continue }
    $want = (Get-Content -Raw $sidecar).Trim().Split()[0]
    $got  = (Get-FileHash -Algorithm SHA256 -LiteralPath $dest).Hash
    if ($got -ieq $want) {
      Log "SHA-256 check passed for $(Split-Path -Leaf $dest)"
      return $true
    }
    Fail "SHA-256 check failed for $(Split-Path -Leaf $dest)"
  }
  return $false
}

$arch    = Get-Arch
$archive = "share2us-gui_windows_${arch}.zip"

if ($Version -eq 'latest') {
  $hostedUrl = "$($BaseUrl.TrimEnd('/'))/gui/downloads/$archive"
  $githubUrl = "https://github.com/$Repo/releases/latest/download/$archive"
} else {
  $hostedUrl = "$($BaseUrl.TrimEnd('/'))/gui/downloads/$Version/$archive"
  $githubUrl = "https://github.com/$Repo/releases/download/$Version/$archive"
}

$tmp = Join-Path ([System.IO.Path]::GetTempPath()) ("s2u-gui-install-" + [System.IO.Path]::GetRandomFileName())
New-Item -ItemType Directory -Path $tmp -Force | Out-Null
try {
  $archivePath = Join-Path $tmp $archive
  $sidecar     = "$archivePath.sha256"

  Log "Downloading Share2Us GUI for windows/$arch..."
  if (-not (Download-Verified $archivePath $sidecar @($githubUrl, $hostedUrl))) {
    Fail "could not download and verify $archive"
  }

  Expand-Archive -Force -LiteralPath $archivePath -DestinationPath $tmp
  $src = Get-ChildItem -Path $tmp -Recurse -Filter $BinaryName | Select-Object -First 1
  if (-not $src) { Fail "archive did not contain $BinaryName" }

  New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
  $exe = Join-Path $InstallDir $BinaryName
  Copy-Item -Force -LiteralPath $src.FullName -Destination $exe
} finally {
  Remove-Item -Recurse -Force -LiteralPath $tmp -ErrorAction SilentlyContinue
}

# Register the Explorer right-click "s2u -> Share" integration (per-user, no admin).
& $exe --install-shell

# Start Menu shortcut.
try {
  $programs = [Environment]::GetFolderPath('Programs')
  $lnk = Join-Path $programs 'Share2Us.lnk'
  $ws = New-Object -ComObject WScript.Shell
  $sc = $ws.CreateShortcut($lnk)
  $sc.TargetPath = $exe
  $sc.Save()
} catch { }

Log ""
Log "Installed Share2Us to $InstallDir\$BinaryName"
Log "Right-click a file or folder in Explorer -> s2u -> Share."
Log "(On Windows 11, the entry is under 'Show more options'.)"
Log "Sign in from the app the first time."
