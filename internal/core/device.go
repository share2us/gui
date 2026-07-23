package core

import (
	"context"
	"strings"

	clicore "github.com/share2us/cli-core"
)

// Device is one of the account's own device sessions, offered as a send target.
type Device struct {
	SessionID string // device_session_id — the send target
	Name      string // device name (usually the hostname)
	Label     string // "<name>:<os>" for display, e.g. "openclaw:linux"
	PublicKey string // sealed-box target key; "" when the device has no key yet
	HasKey    bool
	Current   bool // true for this device
}

// Devices lists the account's own devices. Only HasKey devices can receive an
// encrypted send; the UI should dim the rest.
func (c *Client) Devices(ctx context.Context) ([]Device, error) {
	resp, err := c.api.ListDevices(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Device, 0, len(resp.Sessions))
	for _, s := range resp.Sessions {
		out = append(out, Device{
			SessionID: s.ID,
			Name:      s.DeviceName,
			Label:     deviceLabel(s),
			PublicKey: s.PublicKey,
			HasKey:    s.PublicKey != "",
			Current:   s.Current,
		})
	}
	return out, nil
}

// deviceLabel renders "<device_name>:<os>" (e.g. "openclaw:linux").
//
// TODO(os): the /v1/devices payload does not currently surface the OS, and
// cli-core's DeviceSession has no OS field — so we fall back to client_type here.
// Adding `OS string json:"os"` to cli-core DeviceSession (and confirming the API
// returns it) will make this label exact.
func deviceLabel(s clicore.DeviceSession) string {
	name := s.DeviceName
	if name == "" {
		name = "device"
	}
	osName := s.ClientType
	if osName == "" {
		osName = "unknown"
	}
	return name + ":" + strings.ToLower(osName)
}
