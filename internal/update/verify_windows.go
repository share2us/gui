//go:build windows && !store

package update

import (
	"errors"
	"os/exec"
	"strings"
	"syscall"
)

// createNoWindow suppresses the console window Windows would otherwise flash when
// a GUI (-H windowsgui) process spawns a console app like powershell.exe.
const createNoWindow = 0x08000000 // CREATE_NO_WINDOW

// VerifySignature returns nil when path carries an Authenticode signature whose
// signer subject names Share2.us. It is FAIL-CLOSED on anything it can check:
// no signature, an unreadable one, or a different signer means "do not run this".
//
// It deliberately does NOT require Status == Valid, and that is not a relaxation
// of a working check — it is the removal of one that could never pass.
//
// The original version required Valid, on the assumption (still visible in an old
// comment) that the installer imported the Share2.us code-signing cert into the
// machine trust store on first run. That import was removed in c1f82ae because
// adding a root CA is a textbook malware behaviour and Defender blocked the
// installer for it. Nothing imports the cert now, so Status reads UnknownError.
//
// It could not be restored by re-adding the import either: the release workflow
// calls New-SelfSignedCertificate on EVERY run, so each release is signed by a
// different, unrelated keypair. There is no stable identity to trust, which is
// also why SmartScreen reputation never accumulates (installer/DISTRIBUTION.md).
//
// Verified on Windows 10 Pro 2026-09-02: both the shipped Setup.exe and the
// installed share2us-gui.exe report UnknownError, and zero Share2.us certs exist
// in any trust store. Every update offered to a Windows user was therefore
// refused at this gate.
//
// Integrity is now carried by VerifyChecksum against the release's published
// .sha256. This function is left as a cheap "is it still signed by us" smoke
// test. A real identity check needs a stable signing key held outside CI.
func VerifySignature(path string) error {
	script := "$ErrorActionPreference='Stop';" +
		"$s=Get-AuthenticodeSignature -LiteralPath " + psQuote(path) + ";" +
		"Write-Output $s.Status;" +
		"if($s.SignerCertificate){Write-Output $s.SignerCertificate.Subject}else{Write-Output ''}"
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
	out, err := cmd.Output()
	if err != nil {
		return errors.New("could not verify the update's code signature")
	}
	lines := strings.Split(strings.ReplaceAll(string(out), "\r\n", "\n"), "\n")
	status, subject := "", ""
	if len(lines) > 0 {
		status = strings.TrimSpace(lines[0])
	}
	if len(lines) > 1 {
		subject = strings.TrimSpace(lines[1])
	}
	// NotSigned / HashMismatch mean the file is unsigned or altered: refuse.
	// UnknownError is what a self-signed cert legitimately produces here, so it
	// is accepted (see the doc comment) — the checksum is the real gate.
	switch status {
	case "NotSigned", "HashMismatch":
		return errors.New("the update is not signed (" + status + ")")
	case "":
		return errors.New("could not read the update's code signature")
	}
	if !strings.Contains(strings.ToLower(subject), "share2.us") {
		return errors.New("the update is not signed by Share2.us")
	}
	return nil
}

// psQuote wraps s as a single-quoted PowerShell literal (doubling any quote).
func psQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
