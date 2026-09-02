//go:build !store

package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
)

// maxChecksumBody bounds the .sha256 fetch. The file is one hex line; anything
// larger is not ours.
const maxChecksumBody = 4 << 10

// VerifyChecksum reports whether the file at path matches the SHA-256 published
// alongside the release asset at checksumURL.
//
// This is the integrity gate for the Windows self-update, and it exists because
// Authenticode cannot be one here: the release workflow mints a NEW self-signed
// certificate on every run, so the signature chains to nothing, its thumbprint
// differs per release, and Get-AuthenticodeSignature reports UnknownError rather
// than Valid on any machine. VerifySignature used to require Valid and therefore
// rejected every update that was ever offered.
//
// Be honest about what this proves: the installer and its hash are published by
// the same release, fetched over HTTPS from a host allowlist. It defends against
// a corrupted or truncated download and against tampering in transit. It does
// NOT defend against a compromised release, which needs a stable signing key the
// project does not yet have. The CLI's update path makes the same trade.
//
// Fail-closed: any error, a missing checksum, or a mismatch means do not run it.
func VerifyChecksum(ctx context.Context, client *http.Client, path, checksumURL string) error {
	if strings.TrimSpace(checksumURL) == "" {
		return errors.New("this release publishes no checksum for the update; install it manually")
	}
	want, err := fetchChecksum(ctx, client, checksumURL)
	if err != nil {
		return err
	}
	got, err := fileSHA256(path)
	if err != nil {
		return err
	}
	if !strings.EqualFold(got, want) {
		return errors.New("the downloaded update does not match its published checksum")
	}
	return nil
}

func fetchChecksum(ctx context.Context, client *http.Client, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "share2us-gui")
	resp, err := client.Do(req)
	if err != nil {
		return "", errors.New("could not fetch the update's checksum")
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", errors.New("could not fetch the update's checksum")
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxChecksumBody))
	if err != nil {
		return "", err
	}
	// Accept both a bare hash and `<hash>  <filename>` (sha256sum output).
	sum := strings.TrimSpace(string(raw))
	if i := strings.IndexAny(sum, " \t\r\n"); i > 0 {
		sum = sum[:i]
	}
	if len(sum) != 64 {
		return "", errors.New("the update's published checksum is malformed")
	}
	if _, err := hex.DecodeString(sum); err != nil {
		return "", errors.New("the update's published checksum is malformed")
	}
	return sum, nil
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
