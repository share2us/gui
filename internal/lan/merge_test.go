// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Hassan Khurram

package lan

import (
	"testing"

	"github.com/share2us/cli-core/lanshare"
)

func mdnsPeer(name, host, fp string) lanshare.Peer {
	return lanshare.Peer{Name: name, Host: host, Port: 4300, Fingerprint: fp, Mode: lanshare.ModeOpen}
}

// A device reachable by both methods must appear once, described by the source
// that actually knows its name.
func TestMergePrefersTheNamedEntry(t *testing.T) {
	out := mergePeers(
		[]lanshare.Peer{mdnsPeer("kestrel", "192.168.15.9", "AA")},
		[]lanshare.ScannedPeer{{Host: "192.168.15.9", Port: 4300, Fingerprint: "AA"}},
		nil,
	)
	if len(out) != 1 {
		t.Fatalf("got %d peers, want 1 — the same device was listed twice", len(out))
	}
	if out[0].Name != "kestrel" {
		t.Errorf("Name = %q, want the announced name", out[0].Name)
	}
	if out[0].ViaScan {
		t.Error("a device mDNS described should not be marked as scan-found")
	}
}

// The whole point of the scan path: it finds devices on a network that drops
// multicast, where mDNS returns nothing at all.
func TestMergeKeepsScanOnlyDevices(t *testing.T) {
	out := mergePeers(nil, []lanshare.ScannedPeer{{Host: "192.168.15.20", Port: 4300, Fingerprint: "BB"}}, nil)
	if len(out) != 1 {
		t.Fatalf("got %d peers, want 1", len(out))
	}
	if !out[0].ViaScan {
		t.Error("must be marked scan-found so the UI can explain the missing name")
	}
	// No name arrived. Showing the address is honest; inventing a label is not.
	if out[0].Name != "192.168.15.20" {
		t.Errorf("Name = %q, want the address", out[0].Name)
	}
	if out[0].Code == "" {
		t.Error("a scan-found device still needs its verify code: identity is confirmed the same way")
	}
	if out[0].Dest == "" {
		t.Error("a scan-found device must be addressable, or listing it is pointless")
	}
}

// A peer that announced no fingerprint cannot be deduplicated or verified, but
// it must not knock out other entries by colliding on an empty key.
func TestMergeKeepsSeveralUnidentifiedPeers(t *testing.T) {
	out := mergePeers(
		[]lanshare.Peer{mdnsPeer("one", "10.0.0.1", ""), mdnsPeer("two", "10.0.0.2", "")},
		nil,
		nil,
	)
	if len(out) != 2 {
		t.Fatalf("got %d peers, want 2 — an empty fingerprint must not collide", len(out))
	}
}

// A scanned peer with no fingerprint proved nothing about who it is, so it is
// not a Share2Us device we can address.
func TestMergeDropsScannedPeerWithoutIdentity(t *testing.T) {
	out := mergePeers(nil, []lanshare.ScannedPeer{{Host: "10.0.0.5", Port: 4300}}, nil)
	if len(out) != 0 {
		t.Fatalf("got %d peers, want 0", len(out))
	}
}

// Two devices, one of each kind, both survive.
func TestMergeCombinesBothSources(t *testing.T) {
	out := mergePeers(
		[]lanshare.Peer{mdnsPeer("kestrel", "192.168.15.9", "AA")},
		[]lanshare.ScannedPeer{{Host: "192.168.15.20", Port: 4300, Fingerprint: "BB"}},
		nil,
	)
	if len(out) != 2 {
		t.Fatalf("got %d peers, want 2", len(out))
	}
	// The named one first: a list that reorders itself between refreshes is
	// harder to use than one that groups what it knows about.
	if out[0].Name != "kestrel" {
		t.Errorf("first entry = %q, want the named device", out[0].Name)
	}
}

// A device reached over the tailnet is not "nearby" in any physical sense, and
// the UI needs to be able to say so.
func TestMergeMarksTailnetPeers(t *testing.T) {
	out := mergePeers(nil, []lanshare.ScannedPeer{{Host: "100.100.25.200", Port: 4300, Fingerprint: "CC", ViaTailscale: true}}, nil)
	if len(out) != 1 || !out[0].ViaTailscale {
		t.Fatal("a tailnet peer must be marked as one")
	}
}

// Neither source finding anything is an empty list, not a nil surprise.
func TestMergeOfNothingIsEmpty(t *testing.T) {
	if out := mergePeers(nil, nil, nil); len(out) != 0 {
		t.Fatalf("got %d peers, want 0", len(out))
	}
}

// The name from a peer's verified card, rather than the bare address that was
// all a TLS probe could offer before.
func TestMergeUsesTheNameFromAScannedPeersCard(t *testing.T) {
	out := mergePeers(nil, []lanshare.ScannedPeer{{
		Host: "192.168.15.20", Port: 4300, Fingerprint: "BB",
		Name: "kestrel", IdentityFingerprint: "ID-K",
	}}, nil)
	if len(out) != 1 {
		t.Fatalf("got %d peers, want 1", len(out))
	}
	if out[0].Name != "kestrel" {
		t.Errorf("Name = %q, want the name from the card", out[0].Name)
	}
	if out[0].Identity != "ID-K" {
		t.Errorf("Identity = %q, want the stable fingerprint", out[0].Identity)
	}
	if out[0].Address != "192.168.15.20" {
		t.Errorf("Address = %q, want the bare host alongside the name", out[0].Address)
	}
}

// A peer on an older build publishes no card. It must degrade to exactly the old
// behaviour rather than showing an empty name.
func TestMergeFallsBackToTheAddressWithoutACard(t *testing.T) {
	out := mergePeers(nil, []lanshare.ScannedPeer{{Host: "192.168.15.20", Port: 4300, Fingerprint: "BB"}}, nil)
	if out[0].Name != "192.168.15.20" {
		t.Errorf("Name = %q, want the address", out[0].Name)
	}
	if out[0].Identity != "" {
		t.Errorf("Identity = %q, want empty — nothing proved one", out[0].Identity)
	}
}

// The user's own name beats the one the device chose for itself.
func TestMergeAppliesTheUsersOwnName(t *testing.T) {
	aliases := map[string]string{"ID-K": "Study laptop"}
	out := mergePeers(nil, []lanshare.ScannedPeer{{
		Host: "192.168.15.20", Port: 4300, Fingerprint: "BB",
		Name: "DESKTOP-4F2K9A", IdentityFingerprint: "ID-K",
	}}, aliases)
	if out[0].Name != "Study laptop" {
		t.Errorf("Name = %q, want the user's own name", out[0].Name)
	}
	if !out[0].Aliased {
		t.Error("the UI must be able to say the name is the user's, not the device's")
	}
}

// An alias applies to a device found by mDNS too, or the same machine would be
// named on one refresh and not the next depending on how it was discovered.
func TestMergeAppliesAnAliasToAnMDNSPeerToo(t *testing.T) {
	out := mergePeers(
		[]lanshare.Peer{mdnsPeer("DESKTOP-4F2K9A", "192.168.15.9", "AA")},
		[]lanshare.ScannedPeer{{Host: "192.168.15.9", Port: 4300, Fingerprint: "AA", Name: "kestrel", IdentityFingerprint: "ID-K"}},
		map[string]string{"ID-K": "Study laptop"},
	)
	if out[0].Name != "Study laptop" {
		t.Errorf("Name = %q, want the user's own name", out[0].Name)
	}
}

// An alias is keyed on identity. A peer that proved none must never pick one up,
// or a name would attach to whatever holds that address next.
func TestMergeNeverAliasesAPeerWithNoIdentity(t *testing.T) {
	out := mergePeers(nil, []lanshare.ScannedPeer{{Host: "192.168.15.20", Port: 4300, Fingerprint: "BB"}},
		map[string]string{"": "Study laptop"})
	if out[0].Aliased {
		t.Fatal("a peer with no proven identity was given a name")
	}
	if out[0].Name != "192.168.15.20" {
		t.Errorf("Name = %q, want the address", out[0].Name)
	}
}

// The card belongs to the device, not to whichever method found it. mDNS wins
// the dedupe, so without carrying the scan's verified identity across, anything
// on the local segment — the common case — would be listed from the source that
// has no proven identity and could be neither remembered nor named.
func TestMergeCarriesAVerifiedCardOntoTheMDNSEntry(t *testing.T) {
	out := mergePeers(
		[]lanshare.Peer{mdnsPeer("kestrel", "192.168.15.9", "AA")},
		[]lanshare.ScannedPeer{{Host: "192.168.15.9", Port: 4300, Fingerprint: "AA", Name: "kestrel", IdentityFingerprint: "ID-K"}},
		nil,
	)
	if len(out) != 1 {
		t.Fatalf("got %d peers, want 1", len(out))
	}
	if out[0].Identity != "ID-K" {
		t.Errorf("Identity = %q, want the identity the probe verified", out[0].Identity)
	}
}

// An mDNS TXT record is attacker-choosable; a card is signed. When they disagree
// about the same device, the proven name is the one to show.
func TestMergePrefersTheSignedNameOverTheAnnouncedOne(t *testing.T) {
	out := mergePeers(
		[]lanshare.Peer{mdnsPeer("Hassan's Laptop", "192.168.15.9", "AA")},
		[]lanshare.ScannedPeer{{Host: "192.168.15.9", Port: 4300, Fingerprint: "AA", Name: "kestrel", IdentityFingerprint: "ID-K"}},
		nil,
	)
	if out[0].Name != "kestrel" {
		t.Errorf("Name = %q, want the name that was actually signed", out[0].Name)
	}
}
