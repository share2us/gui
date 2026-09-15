// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Hassan Khurram

package core

import (
	"context"
	"io"
	"os"

	clicore "github.com/share2us/cli-core"
	"github.com/share2us/cli-core/lanid"
	"github.com/share2us/cli-core/lanshare"
)

// Local-first routing for device sends (ADR-040), the desktop half of what the
// CLI does in localroute.go. The decision itself lives in cli-core so both
// clients make the SAME one — a routing rule implemented twice diverges.
//
// Sending to one of your own machines used to mean uploading the file and having
// the other end download it, spending quota twice, even with that machine on the
// same network. This hands it straight across when it can.

// Seams for the tests: both reach the network, and what is worth pinning is the
// decision, not the transfer.
var (
	matchLocalDevices = clicore.MatchLocalDevices
	lanSend           = lanshare.Send
)

// routeLocally splits targets into those answering on this network right now and
// those that are not. It never returns an error: every "cannot" leaves the target
// in the cloud list, because a device answers a probe only while it is actually
// listening ("discoverable on local network", or the daemon), so not-reachable is
// the ordinary case rather than a fault.
func routeLocally(ctx context.Context, targets []DeviceTarget) (local []localTarget, cloud []DeviceTarget) {
	refs := make([]clicore.DeviceRef, 0, len(targets))
	bySession := make(map[string]DeviceTarget, len(targets))
	for _, t := range targets {
		bySession[t.SessionID] = t
		if t.LanFingerprint != "" {
			refs = append(refs, clicore.DeviceRef{SessionID: t.SessionID, LanFingerprint: t.LanFingerprint})
		}
	}
	if len(refs) == 0 {
		return nil, targets
	}

	matches, err := matchLocalDevices(ctx, refs, clicore.MatchOptions{})
	if err != nil || len(matches) == 0 {
		return nil, targets
	}

	matched := make(map[string]lanshare.ScannedPeer, len(matches))
	for _, m := range matches {
		matched[m.SessionID] = m.Peer
	}
	for _, t := range targets {
		if peer, ok := matched[t.SessionID]; ok {
			local = append(local, localTarget{Target: t, Peer: peer})
			continue
		}
		cloud = append(cloud, t)
	}
	return local, cloud
}

// localTarget is one device that answered, with the peer that answered for it.
type localTarget struct {
	Target DeviceTarget
	Peer   lanshare.ScannedPeer
}

// sendDirect streams path to one matched peer.
//
// THE PIN IS THE AUTHENTICATION, which is why this needs no password. The scan
// completed a TLS handshake and read a device card out of the peer's certificate,
// and a card is only returned when its signature covers THAT certificate's own
// public key — so a verified card binds the identity key to that exact
// certificate. The identity matched one of this account's devices, so pinning the
// fingerprint from the same handshake gives an authenticated channel to that
// device and nothing else. Without the pin, TLS would accept any certificate and
// an on-path attacker on the LAN could take the transfer.
func sendDirect(ctx context.Context, path string, lt localTarget, onProgress func(sent, total int64)) error {
	name, size, isDir, readPath, cleanup, err := PrepareLocal(path)
	defer cleanup()
	if err != nil {
		return err
	}
	f, err := os.Open(readPath)
	if err != nil {
		return err
	}
	defer f.Close()

	opts := lanshare.SendOptions{
		Dest:           lt.Peer.Addr(),
		PinFingerprint: lt.Peer.Fingerprint,
		OnProgress:     onProgress,
	}
	// Present this device's identity so the receiver can recognise it. Without
	// one a trusted-sender receiver would refuse, so this is not best-effort here
	// the way it is for an anonymous LAN send.
	id, ierr := lanid.Identity()
	if ierr != nil {
		return ierr
	}
	opts.Identity = id
	if host, herr := os.Hostname(); herr == nil {
		opts.SenderName = host
	}
	_, err = lanSend(ctx, name, size, isDir, io.Reader(f), opts)
	return err
}

// CloudFallback describes a device send that is about to leave this machine,
// so the app can put the cost in front of the user BEFORE it happens.
//
// The owner's requirement (2026-09-12) is that a user must know that cloud
// transfer and retention are the paid parts: "add option (keep in cloud
// [storage counts towards your quota]) to user so user will know". A silent
// upload is the thing that requirement exists to prevent, so this is a question
// and not a notification.
type CloudFallback struct {
	// DeviceNames are the devices whose copy will be uploaded, in the order the
	// user picked them.
	DeviceNames []string `json:"deviceNames"`
	// SizeBytes is what will be stored against the quota. ONE upload covers every
	// device in the list -- the content key is sealed per device -- so this is the
	// file's size, not the size times the number of devices, and the dialog must
	// not imply otherwise.
	SizeBytes int64 `json:"sizeBytes"`
	// DirectWasPossible is true when at least one of these devices publishes a LAN
	// fingerprint, meaning it COULD have taken the file directly and simply was not
	// listening. Only then is "turn on discoverable" advice the user can act on;
	// for a device that can never be matched it would be the same fault as telling
	// someone to install the app on their browser.
	DirectWasPossible bool `json:"directWasPossible"`
}

// ConfirmCloud is asked before a device send falls back to uploading. Returning
// false cancels the send.
//
// Nil means "do not ask", which is what every non-interactive caller wants and
// what keeps the tests and the CLI path unchanged. A prompt that cannot be
// answered must never be able to block a transfer.
type ConfirmCloud func(CloudFallback) bool
