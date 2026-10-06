// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Hassan Khurram

package core

import (
	"context"
	"errors"
	"strings"

	clicore "github.com/share2us/cli-core"
)

// AgentSession is one reachable coding-agent session on the account, as the
// desktop app shows it. Device/signing keys are never exposed to the frontend.
type AgentSession struct {
	AgentID    string `json:"agentId"`
	SessionID  string `json:"sessionId"`
	DeviceID   string `json:"deviceId"`
	DeviceName string `json:"deviceName"`
	Tool       string `json:"tool"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	LastSeen   string `json:"lastSeen"`
}

// AgentSessions lists the account's reachable agent sessions across the user's
// devices (the same directory `s2u agent list` shows).
func (c *Client) AgentSessions(ctx context.Context) ([]AgentSession, error) {
	list, err := c.api.ListAgentSessions(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]AgentSession, 0, len(list))
	for _, s := range list {
		out = append(out, AgentSession{
			AgentID: s.AgentID, SessionID: s.SessionID, DeviceID: s.DeviceID,
			DeviceName: s.DeviceName, Tool: s.Tool, Name: s.Name,
			Status: s.Status, LastSeen: s.LastSeen,
		})
	}
	return out, nil
}

// SendToAgentResult is what the frontend gets back after a send.
type SendToAgentResult struct {
	RequestID string `json:"requestId"`
	Status    string `json:"status"`    // "queued" | "pending"
	Transport string `json:"transport"` // "direct LAN" | "relay" | ""
	Busy      bool   `json:"busy"`
}

// SendToAgent sends a file (and/or prompt) to one of the account's agent sessions,
// chosen by agent id (preferred) or session id. inbox=true drops the file into the
// agent's inbox without running a prompt. The file goes directly over LAN/Tailscale
// when the target is reachable, else over the relay (reported in Transport).
func (c *Client) SendToAgent(ctx context.Context, agentID, sessionID, filePath, prompt string, inbox bool) (SendToAgentResult, error) {
	list, err := c.api.ListAgentSessions(ctx)
	if err != nil {
		return SendToAgentResult{}, err
	}
	var target *clicore.AgentSessionInfo
	for i := range list {
		s := &list[i]
		if s.Status == "offline" {
			continue
		}
		if (agentID != "" && s.AgentID == agentID) || (agentID == "" && sessionID != "" && s.SessionID == sessionID) {
			target = s
			break
		}
	}
	if target == nil {
		return SendToAgentResult{}, errors.New("that agent is not reachable right now")
	}
	if strings.TrimSpace(target.DevicePublicKey) == "" {
		return SendToAgentResult{}, errors.New("that agent's device has no encryption key yet, so it cannot receive a sealed file")
	}
	if err := c.ensureSigningKey(ctx); err != nil {
		return SendToAgentResult{}, err
	}
	res, err := c.api.SendAgentFile(ctx, c.cred, clicore.SendAgentFileParams{
		TargetDeviceID:       target.DeviceID,
		TargetSessionID:      target.SessionID,
		Tool:                 target.Tool,
		TargetPublicKey:      target.DevicePublicKey,
		TargetLANFingerprint: target.LANFingerprint,
		TargetAgentID:        target.AgentID,
		Prompt:               prompt,
		FilePath:             filePath,
		Inbox:                inbox,
		SenderDeviceName:     c.currentDeviceName(ctx),
	})
	if err != nil {
		return SendToAgentResult{}, err
	}
	return SendToAgentResult{RequestID: res.RequestID, Status: res.Status, Transport: res.Transport, Busy: res.Busy}, nil
}

// AgentStatus is the current state of a sent hop: its status and, once the agent
// has run, its reply (Result). Empty Result until there is one.
type AgentStatus struct {
	RequestID string `json:"requestId"`
	Status    string `json:"status"`
	Result    string `json:"result"`
}

// AgentStatus polls one sent hop's state, so the app can show pending -> running
// -> done/failed and the agent's reply.
func (c *Client) AgentStatus(ctx context.Context, requestID string) (AgentStatus, error) {
	st, err := c.api.AgentInjectStatus(ctx, strings.TrimSpace(requestID))
	if err != nil {
		return AgentStatus{}, err
	}
	return AgentStatus{RequestID: st.ID, Status: st.Status, Result: st.Result}, nil
}

// ensureSigningKey makes sure this device has a hop signing key and that the
// server holds it, mirroring the CLI: the key is saved locally before it is
// registered (the server treats it as write-once). Idempotent.
func (c *Client) ensureSigningKey(ctx context.Context) error {
	if strings.TrimSpace(c.cred.DeviceSessionID) == "" {
		return errors.New("this login has no device session; sign out and in again to send to agents")
	}
	if c.cred.DeviceSigningPrivateKey == "" || c.cred.DeviceSigningPublicKey == "" {
		kp, err := clicore.NewSigningKeyPair()
		if err != nil {
			return err
		}
		c.cred.DeviceSigningPublicKey = kp.PublicKey
		c.cred.DeviceSigningPrivateKey = kp.PrivateKey
		if err := clicore.SaveCredential(c.cred); err != nil {
			return err
		}
	}
	if err := c.api.RegisterSigningKey(ctx, c.cred.DeviceSigningPublicKey); err != nil {
		if strings.Contains(err.Error(), "signing_key_already_set") {
			return errors.New("the server holds a different send key for this device; sign out and in again")
		}
		return err
	}
	return nil
}

// currentDeviceName is this device's display name for the recipient, best-effort.
func (c *Client) currentDeviceName(ctx context.Context) string {
	resp, err := c.api.ListDevices(ctx)
	if err != nil {
		return ""
	}
	for _, d := range resp.Sessions {
		if (c.cred.DeviceSessionID != "" && d.ID == c.cred.DeviceSessionID) || d.Current {
			return strings.TrimSpace(d.DeviceName)
		}
	}
	return ""
}
