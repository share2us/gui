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
// pinnedThumbprint is set by SetPinnedThumbprint at startup from the value CI
// stamped into the binary. Empty means "not pinned" — older builds, forks, and
// any release cut without a stable signing certificate.
var pinnedThumbprint string

// SetPinnedThumbprint records the certificate this build was signed with.
func SetPinnedThumbprint(t string) { pinnedThumbprint = strings.TrimSpace(t) }

func VerifySignature(path string) error {
	script := "$ErrorActionPreference='Stop';" +
		"$s=Get-AuthenticodeSignature -LiteralPath " + psQuote(path) + ";" +
		"Write-Output $s.Status;" +
		"if($s.SignerCertificate){Write-Output $s.SignerCertificate.Subject}else{Write-Output ''};" +
		"if($s.SignerCertificate){Write-Output $s.SignerCertificate.Thumbprint}else{Write-Output ''}"
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
	out, err := cmd.Output()
	if err != nil {
		return errors.New("could not verify the update's code signature")
	}
	lines := strings.Split(strings.ReplaceAll(string(out), "\r\n", "\n"), "\n")
	status, subject, thumb := "", "", ""
	if len(lines) > 0 {
		status = strings.TrimSpace(lines[0])
	}
	if len(lines) > 1 {
		subject = strings.TrimSpace(lines[1])
	}
	if len(lines) > 2 {
		thumb = strings.TrimSpace(lines[2])
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
	// When this build was signed with the stable certificate, require the update
	// to carry the SAME certificate. That is the publisher continuity a
	// per-release self-signed cert can never provide: an update signed by any
	// other key is refused even though its subject also says Share2.us.
	//
	// Rotating the signing certificate therefore breaks self-update for already
	// installed clients — they pin the old thumbprint. Ship a transition (sign
	// with both, or expect a manual reinstall) before rotating.
	if !thumbprintAccepted(pinnedThumbprint, thumb) {
		return errors.New("the update is signed by a different certificate than this build")
	}
	return nil
}

// psQuote wraps s as a single-quoted PowerShell literal (doubling any quote).
func psQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
