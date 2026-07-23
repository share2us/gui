# Run elevated by the installer (Tasks: sharesheet). Trusts the bundled
# self-signed cert so Windows shows publisher "Share2.us" (instead of "unknown
# publisher"), then installs the MSIX so S2u appears in the Windows Share sheet
# (Snip & Sketch's "Share").
$ErrorActionPreference = 'SilentlyContinue'
$dir = $PSScriptRoot
$cert = Get-ChildItem (Join-Path $dir '*.cer') | Select-Object -First 1
$msix = Get-ChildItem (Join-Path $dir '*.msix') | Select-Object -First 1

if ($cert) {
  foreach ($store in 'Root', 'TrustedPublisher', 'TrustedPeople') {
    Import-Certificate -FilePath $cert.FullName -CertStoreLocation "Cert:\LocalMachine\$store" | Out-Null
  }
}
if ($msix) {
  Add-AppxPackage -Path $msix.FullName -ForceApplicationShutdown
}
