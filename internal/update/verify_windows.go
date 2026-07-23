//go:build windows

package update

import (
	"errors"
	"os/exec"
	"strings"
)

// VerifySignature returns nil only when path carries a Valid Authenticode
// signature whose signer subject names Share2.us. It is FAIL-CLOSED: any error,
// a non-Valid status, or a different signer means "do not run this update".
//
// Get-AuthenticodeSignature validates against the machine trust store; the
// Share2.us code-signing cert is trusted there after the first installer run
// imports it, so a legitimately-signed update reads as Valid while a swapped or
// unsigned binary does not. This is the gate that stops the auto-updater from
// executing an attacker-supplied installer even if the download were tampered.
func VerifySignature(path string) error {
	script := "$ErrorActionPreference='Stop';" +
		"$s=Get-AuthenticodeSignature -LiteralPath " + psQuote(path) + ";" +
		"Write-Output $s.Status;" +
		"if($s.SignerCertificate){Write-Output $s.SignerCertificate.Subject}else{Write-Output ''}"
	out, err := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script).Output()
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
	if status != "Valid" {
		return errors.New("the update's code signature is not Valid (" + status + ")")
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
