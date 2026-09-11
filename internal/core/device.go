// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Hassan Khurram

package core

import (
	"context"
	"strings"

	clicore "github.com/share2us/cli-core"
)

// Device is one of the account's own device sessions, offered as a send target.
// Device crosses to the frontend, so the json tags are not decoration: without
// them Wails marshals the Go field names (SessionID, Name, ...) and the UI, which
// reads sessionId/name/..., gets undefined for every field. That is what happened
// -- see TestDeviceJSONMatchesTheFrontendContract, which exists to stop it
// happening again. Every other struct on this boundary is tagged; this one was
// the exception.
type Device struct {
	SessionID string `json:"sessionId"` // device_session_id — the send target
	Name      string `json:"name"`      // device name (usually the hostname)
	Label     string `json:"label"`     // "<name>:<os>" for display, e.g. "openclaw:linux"
	PublicKey string `json:"publicKey"` // sealed-box target key; "" when the device has no key yet
	HasKey    bool   `json:"hasKey"`
	Current   bool   `json:"current"` // true for this device
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
		// A signed-in BROWSER is not a device and is left out entirely.
		//
		// It has no device keypair and cannot be given one, so it can never
		// receive a sealed send. It used to sit in this list reading "can't
		// receive yet - sign in with Share2Us on it", which is advice nobody can
		// follow: there is nothing to install on Chrome. The app has no action
		// that applies to a browser session either -- it cannot sign one out --
		// so the row was noise carrying an impossible instruction.
		//
		// Sharing a file with yourself in a browser is what a link is for, and
		// what "Only me" was built for.
		if isBrowser(s.ClientType) {
			continue
		}
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

// isBrowser reports a session that is a web browser rather than a machine running
// the app. The CLI makes the same distinction, and for the same reason.
func isBrowser(clientType string) bool {
	return strings.EqualFold(strings.TrimSpace(clientType), "web")
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
