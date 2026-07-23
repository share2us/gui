// Package core is the platform-independent heart of the Share2Us Windows app.
// It wraps the reusable cli-core library (auth, upload, crypto, inbox) so the
// Wails UI and the tray receiver share one implementation and stay faithful to
// the CLI's behaviour. Nothing in this package imports Wails or Windows APIs, so
// it builds and unit-tests on any OS.
package core

import (
	"errors"
	"fmt"

	clicore "github.com/share2us/cli-core"
)

// ErrNotLoggedIn is returned when there is no saved login to act on.
var ErrNotLoggedIn = errors.New("not logged in")

// Client is an authenticated Share2Us session bound to the saved credential.
type Client struct {
	api  *clicore.Client
	cred clicore.Credential
}

// Load builds a Client from the saved credential. This is the exact same store
// the CLI uses: %AppData%\share2us\credentials.json on Windows (see cli-core
// CredentialPath). Returns ErrNotLoggedIn when there is no usable login.
func Load() (*Client, error) {
	cred, err := clicore.LoadCredential()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNotLoggedIn, err)
	}
	if cred.Token == "" {
		return nil, ErrNotLoggedIn
	}
	return &Client{api: clicore.NewClient(cred.APIBase, cred.Token), cred: cred}, nil
}

// Logout deletes the saved credential (the shared cli-core store), signing out
// of both the app and the CLI on this machine.
func Logout() error { return clicore.DeleteCredential() }

// Email is the logged-in account's email.
func (c *Client) Email() string { return c.cred.Email }

// IsAPIToken reports whether the saved token is a non-interactive personal API
// token, which cannot perform device/contact end-to-end encryption (it has no
// device keypair). The UI should disable device/contact targets in that case.
func (c *Client) IsAPIToken() bool { return clicore.IsAPIToken(c.cred.Token) }

// HasDeviceKey reports whether this login carries a device keypair, required to
// receive (decrypt) sealed inbox shares.
func (c *Client) HasDeviceKey() bool {
	return c.cred.DevicePublicKey != "" && c.cred.DevicePrivateKey != ""
}
