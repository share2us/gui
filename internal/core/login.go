package core

import (
	"context"
	"errors"
	"strings"
	"time"

	clicore "github.com/share2us/cli-core"
)

// LoginSession is an in-progress device-code login. StartLogin returns one with
// the user code and verification URL for the UI to show/open; Wait then blocks
// until the user approves in the browser. Ported from the CLI's login().
type LoginSession struct {
	apiBase         string
	deviceCode      string
	interval        int
	UserCode        string // short code the user confirms in the browser
	VerificationURI string // page to enter the code on
	VerificationURL string // complete URL (code embedded) to open directly
}

// StartLogin begins the device-code flow: it registers a pending device code
// with the API and returns the codes/URL to display. deviceName may be "" to use
// the detected hostname.
func StartLogin(ctx context.Context, deviceName string) (*LoginSession, error) {
	apiBase, _, err := clicore.ResolveAPIBase()
	if err != nil || apiBase == "" {
		apiBase = clicore.DefaultAPIBase
	}
	client := clicore.NewClient(apiBase, "")
	device, err := clicore.DetectDeviceMetadata(deviceName)
	if err != nil {
		return nil, err
	}
	code, err := client.StartDeviceCode(ctx, clicore.DeviceCodeRequest{
		DeviceName:    device.DeviceName,
		MachineID:     device.MachineID,
		OS:            device.OS,
		Arch:          device.Arch,
		ClientVersion: clicore.FullVersion(),
	})
	if err != nil {
		return nil, err
	}
	return &LoginSession{
		apiBase:         apiBase,
		deviceCode:      code.DeviceCode,
		interval:        code.Interval,
		UserCode:        code.UserCode,
		VerificationURI: code.VerificationURI,
		VerificationURL: clicore.VerificationURL(code),
	}, nil
}

// Wait polls until the user approves the login, then generates/reuses this
// machine's device keypair, registers the public key, saves the credential (the
// same store the CLI uses), and returns an authenticated Client. It honours ctx
// cancellation/timeout (device codes expire).
func (s *LoginSession) Wait(ctx context.Context) (*Client, error) {
	client := clicore.NewClient(s.apiBase, "")
	for {
		token, err := client.PollDeviceToken(ctx, s.deviceCode)
		if clicore.IsAuthorizationPending(err) {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(clicore.SleepInterval(s.interval)):
			}
			continue
		}
		if clicore.IsDeviceLimitReached(err) {
			return nil, errors.New("device limit reached — remove a device in the portal (or `s2u devices`) and sign in again")
		}
		if err != nil {
			return nil, err
		}

		authClient := clicore.NewClient(s.apiBase, token.Credential)
		keyPair, err := reuseOrNewDeviceKeyPair()
		if err != nil {
			return nil, err
		}
		if err := authClient.RegisterDeviceKey(ctx, keyPair.PublicKey); err != nil {
			return nil, err
		}
		me, err := authClient.Me(ctx)
		if err != nil {
			return nil, err
		}
		email := me.Email
		if email == "" {
			email = me.UserID
		}
		cred := clicore.Credential{
			APIBase:          s.apiBase,
			Token:            token.Credential,
			Email:            email,
			DeviceSessionID:  token.DeviceSessionID,
			DevicePublicKey:  keyPair.PublicKey,
			DevicePrivateKey: keyPair.PrivateKey,
		}
		if err := clicore.SaveCredential(cred); err != nil {
			return nil, err
		}
		return &Client{api: authClient, cred: cred}, nil
	}
}

// reuseOrNewDeviceKeyPair keeps this machine's X25519 keypair stable across
// re-logins so sealed device/contact shares stay decryptable; a fresh pair is
// generated only on first login. Mirrors the CLI helper of the same name.
func reuseOrNewDeviceKeyPair() (clicore.DeviceKeyPair, error) {
	if cred, err := clicore.LoadCredential(); err == nil {
		if strings.TrimSpace(cred.DevicePublicKey) != "" && strings.TrimSpace(cred.DevicePrivateKey) != "" {
			return clicore.DeviceKeyPair{PublicKey: cred.DevicePublicKey, PrivateKey: cred.DevicePrivateKey}, nil
		}
	}
	return clicore.NewDeviceKeyPair()
}
