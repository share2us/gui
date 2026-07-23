package core

import (
	"context"
	"errors"
	"fmt"
	"os"

	clicore "github.com/share2us/cli-core"
)

// Visibility selects a public or recipient-restricted link share.
type Visibility string

const (
	// Public is anyone-with-the-link.
	Public Visibility = "public"
	// Private restricts the share to the listed recipient emails.
	Private Visibility = "private"
)

// LinkRequest describes a public/private LINK share of a single file.
type LinkRequest struct {
	Path         string
	Visibility   Visibility
	Recipients   []string // Private only: emails allowed to open the share
	Password     string
	OneTime      bool
	Expires      string // "" = server default, e.g. "7d"; ignored when Keep
	Keep         bool   // keep indefinitely (no expiry)
	MaxViews     uint64
	AllowReshare *bool  // Private only: opt into reshare (ADR-024)
	Note         string // short message shown to viewers on the share page
}

// Result is the outcome of a share.
type Result struct {
	PublicID  string
	Link      string
	ExpiresAt string
}

// ShareLink creates a public or private link share. Folders are rejected here
// (the server only accepts folders for device/contact sends) — matching the CLI.
func (c *Client) ShareLink(ctx context.Context, req LinkRequest) (Result, error) {
	p, err := prepareContent(req.Path)
	if err != nil {
		return Result{}, err
	}
	defer p.cleanup()
	if p.isFolder {
		return Result{}, errors.New("folders can only be shared to a device or contact, not as a link")
	}
	apiExpiry, noExpiry, err := resolveExpiry(req.Expires, req.Keep)
	if err != nil {
		return Result{}, err
	}
	sum, err := fileSHA256(p.path)
	if err != nil {
		return Result{}, err
	}
	recipients := req.Recipients
	allowReshare := req.AllowReshare
	if req.Visibility == Public {
		recipients = nil
		allowReshare = nil
	}
	return c.runUpload(ctx, p.path, p.size, clicore.UploadCreateRequest{
		FileName:     p.name,
		SizeBytes:    uint64(p.size),
		ContentType:  p.contentType,
		ContentClass: p.contentClass,
		ExpiresIn:    apiExpiry,
		NoExpiry:     noExpiry,
		SHA256:       sum,
		New:          true,
		Password:     req.Password,
		OneTime:      req.OneTime,
		Recipients:   recipients,
		MaxViews:     req.MaxViews,
		AllowReshare: allowReshare,
		Note:         req.Note,
	})
}

// SendToDevice sends path to one of the account's OWN devices, sealed to that
// device's public key (no trust step: same account). devicePublicKey must be
// non-empty (the device has completed key registration).
func (c *Client) SendToDevice(ctx context.Context, path, deviceSessionID, devicePublicKey string) (Result, error) {
	if c.IsAPIToken() {
		return Result{}, errors.New("device sends need an interactive login, not a personal API token")
	}
	if devicePublicKey == "" {
		return Result{}, errors.New("that device has no encryption key yet; sign in with the app on it first")
	}
	return c.sealedSend(ctx, path, sealedSendOpts{
		targetDevice:    deviceSessionID,
		targetPublicKey: devicePublicKey,
	})
}

// SendToContact sends path to another account addressed by email. It only
// succeeds if the recipient has trusted the sender AND exposed a device to them
// (the server returns just the exposed, key-bearing devices).
func (c *Client) SendToContact(ctx context.Context, path, email string) (Result, error) {
	if c.IsAPIToken() {
		return Result{}, errors.New("contact sends need an interactive login, not a personal API token")
	}
	list, err := c.api.TeammateDevices(ctx, email)
	if err != nil {
		return Result{}, err
	}
	switch list.Code {
	case "recipient_not_registered":
		return Result{}, fmt.Errorf("%s isn't on Share2Us yet", email)
	case "recipient_not_accepting":
		return Result{}, fmt.Errorf("%s isn't accepting files from you", email)
	}
	if len(list.Devices) == 0 {
		return Result{}, fmt.Errorf("%s hasn't shared a device with you yet; ask them to trust you and allow a device", email)
	}
	return c.sealedSend(ctx, path, sealedSendOpts{
		recipientEmail: email,
		contactDevices: list.Devices,
	})
}

// sealedSendOpts selects the sealed-box target: either a single own-device, or a
// contact's fanned-out device list.
type sealedSendOpts struct {
	targetDevice    string
	targetPublicKey string
	recipientEmail  string
	contactDevices  []clicore.TeammateDevice
}

// sealedSend performs the shared encrypt-then-seal upload for device/contact
// sends: fresh data key, stream-encrypt, seal the key per target, upload, and
// retain the key for in-flight re-seal (best effort). Mirrors the CLI's upload().
func (c *Client) sealedSend(ctx context.Context, path string, opts sealedSendOpts) (Result, error) {
	p, err := prepareContent(path)
	if err != nil {
		return Result{}, err
	}
	defer p.cleanup()

	dataKey, err := clicore.NewDataKey()
	if err != nil {
		return Result{}, err
	}
	encPath, encSize, err := encryptToTemp(p.path, dataKey)
	if err != nil {
		return Result{}, err
	}
	defer os.Remove(encPath)

	sum, err := fileSHA256(encPath)
	if err != nil {
		return Result{}, err
	}

	// A sealed non-folder becomes opaque bytes; a folder keeps application/zip so
	// the recipient's client unzips it (matches the CLI).
	contentType := "application/octet-stream"
	if p.isFolder {
		contentType = p.contentType
	}

	req := clicore.UploadCreateRequest{
		FileName:       p.name,
		SizeBytes:      uint64(encSize),
		ContentType:    contentType,
		ContentClass:   p.contentClass,
		SHA256:         sum,
		New:            true,
		Encrypted:      true,
		EncryptionAlgo: clicore.EncryptionAlgoAES256GCM + "+sealedbox",
	}
	switch {
	case opts.targetDevice != "":
		sealed, err := clicore.SealContentKeyForDevice(dataKey, opts.targetPublicKey)
		if err != nil {
			return Result{}, err
		}
		req.TargetDevice = opts.targetDevice
		req.SealedKey = sealed
	default:
		req.RecipientEmail = opts.recipientEmail
		for _, d := range opts.contactDevices {
			sealed, err := clicore.SealContentKeyForDevice(dataKey, d.PublicKey)
			if err != nil {
				return Result{}, err
			}
			req.Targets = append(req.Targets, clicore.UploadTarget{
				TargetDeviceSessionID: d.DeviceID,
				SealedKey:             sealed,
			})
		}
	}

	res, err := c.runUpload(ctx, encPath, encSize, req)
	if err != nil {
		return res, err
	}
	// Best-effort re-seal retention (Option B): keep the content key so we can
	// re-seal it if the recipient re-keys before receiving. Never fatal.
	if len(dataKey) == 32 && res.PublicID != "" {
		_ = clicore.RetainContentKey(res.PublicID, clicore.EncodeKey(dataKey), opts.recipientEmail)
	}
	return res, nil
}

// runUpload is the create -> PUT -> complete pipeline shared by every send.
func (c *Client) runUpload(ctx context.Context, uploadPath string, size int64, req clicore.UploadCreateRequest) (Result, error) {
	created, err := c.api.CreateUpload(ctx, req)
	if err != nil {
		return Result{}, err
	}
	res := Result{
		PublicID:  created.Share.PublicID,
		Link:      firstNonEmpty(created.Link, created.Share.Link),
		ExpiresAt: created.ExpiresAt,
	}
	if !created.SkippedUpload && created.Upload.URL != "" && created.UploadSessionID != "" {
		f, err := os.Open(uploadPath)
		if err != nil {
			return res, err
		}
		defer f.Close()
		if err := c.api.PutUpload(ctx, created.Upload, f, size); err != nil {
			return res, err
		}
		done, err := c.api.CompleteUpload(ctx, created.UploadSessionID)
		if err != nil {
			return res, err
		}
		if done.PublicID != "" {
			res.PublicID = done.PublicID
		}
		if done.ExpiresAt != "" {
			res.ExpiresAt = done.ExpiresAt
		}
	}
	return res, nil
}

// resolveExpiry maps the UI's expiry choice to the API fields. Keep wins and
// sets no-expiry; an empty string leaves the server default.
func resolveExpiry(input string, keep bool) (string, bool, error) {
	if keep {
		return clicore.ExpiryForAPI("none")
	}
	if input == "" {
		return "", false, nil
	}
	return clicore.ExpiryForAPI(input)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
