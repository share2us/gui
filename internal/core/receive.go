package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	clicore "github.com/share2us/cli-core"
)

// Received records one file the receiver saved to disk.
type Received struct {
	PublicID string
	FileName string
	SavedTo  string
	From     string // sender device name, when known
}

// ReceiveOnce polls the inbox once, decrypts every new share with this device's
// key, writes each into destDir (the tray passes the Downloads folder), acks it,
// and returns what was saved. Shares whose sealed key this device cannot open are
// skipped. The inbox only returns approved shares, so untrusted senders never
// reach here (they sit in the pending queue — see Pending).
func (c *Client) ReceiveOnce(ctx context.Context, destDir string) ([]Received, error) {
	if !c.HasDeviceKey() {
		return nil, fmt.Errorf("this login has no device key; sign in interactively to receive")
	}
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return nil, err
	}
	resp, err := c.api.Inbox(ctx)
	if err != nil {
		return nil, err
	}
	var saved []Received
	for _, s := range resp.Shares {
		if s.SealedKey == "" {
			continue // not sealed to a device; nothing this receiver can do
		}
		key, err := clicore.OpenSealedContentKey(s.SealedKey, c.cred.DevicePublicKey, c.cred.DevicePrivateKey)
		if err != nil {
			continue // sealed to a different device key; skip
		}
		dst, err := c.saveInboxShare(ctx, s, key, destDir)
		if err != nil {
			return saved, err
		}
		if err := c.api.AckInboxShare(ctx, s.PublicID); err != nil {
			return saved, err
		}
		saved = append(saved, Received{
			PublicID: s.PublicID,
			FileName: filepath.Base(dst),
			SavedTo:  dst,
			From:     s.FromDeviceName,
		})
	}
	return saved, nil
}

// saveInboxShare downloads the encrypted bytes to a temp file, decrypts them with
// the opened content key, and writes the plaintext into destDir under a unique
// name. Returns the final path.
func (c *Client) saveInboxShare(ctx context.Context, s clicore.InboxShare, key []byte, destDir string) (string, error) {
	tmp, err := os.CreateTemp("", "s2u-inbox-*")
	if err != nil {
		return "", err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if err := c.api.DownloadInboxContent(ctx, s.PublicID, tmp); err != nil {
		_ = tmp.Close()
		return "", err
	}
	if _, err := tmp.Seek(0, 0); err != nil {
		_ = tmp.Close()
		return "", err
	}

	name := filepath.Base(s.FileName)
	if name == "" || name == "." {
		name = s.PublicID
	}
	dst := uniquePath(filepath.Join(destDir, name))
	out, err := os.Create(dst)
	if err != nil {
		_ = tmp.Close()
		return "", err
	}
	if err := clicore.DecryptStream(out, tmp, key); err != nil {
		_ = out.Close()
		_ = tmp.Close()
		_ = os.Remove(dst)
		return "", err
	}
	_ = tmp.Close()
	if err := out.Close(); err != nil {
		_ = os.Remove(dst)
		return "", err
	}
	return dst, nil
}

// Pending lists shares waiting for the receiver's approval (untrusted senders,
// approvals mode). The tray surfaces these as trust prompts.
func (c *Client) Pending(ctx context.Context) ([]clicore.PendingInboxShare, error) {
	resp, err := c.api.ListPendingInbox(ctx)
	if err != nil {
		return nil, err
	}
	return resp.Shares, nil
}
