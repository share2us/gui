package core

import (
	"context"
	"errors"
	"io"
	"os"
	"testing"

	clicore "github.com/share2us/cli-core"
	"github.com/share2us/cli-core/lanshare"
)

// The routing DECISION, which must match what the CLI does — that is why the
// matching itself lives in cli-core. What is pinned here is the split: which
// targets go direct, which fall back to one cloud upload, and that nothing is
// ever lost between the two.

const guiFP = "aaaaaaaabbbbbbbbccccccccddddddddeeeeeeeeffffffff0000000011111111"
const guiFP2 = "1111111100000000ffffffffeeeeeeeeddddddddccccccccbbbbbbbbaaaaaaaa"

func withMatch(t *testing.T, f func(context.Context, []clicore.DeviceRef, clicore.MatchOptions) ([]clicore.LocalMatch, error)) {
	t.Helper()
	old := matchLocalDevices
	matchLocalDevices = f
	t.Cleanup(func() { matchLocalDevices = old })
}

func TestTargetsWithNoFingerprintAllGoToTheCloud(t *testing.T) {
	scanned := false
	withMatch(t, func(context.Context, []clicore.DeviceRef, clicore.MatchOptions) ([]clicore.LocalMatch, error) {
		scanned = true
		return nil, nil
	})
	local, cloud := routeLocally(t.Context(), []DeviceTarget{{SessionID: "a"}, {SessionID: "b"}})
	if len(local) != 0 || len(cloud) != 2 {
		t.Fatalf("local=%d cloud=%d, want 0/2", len(local), len(cloud))
	}
	if scanned {
		t.Error("scanned the network when nothing in the list could be matched")
	}
}

// The mixed case is the one worth getting right: one machine in the room, one
// elsewhere. The near one must go direct and the far one must still be uploaded
// — and exactly once, which is what sealedSend already does for the remainder.
func TestAMixedSendSplitsAndLosesNothing(t *testing.T) {
	withMatch(t, func(context.Context, []clicore.DeviceRef, clicore.MatchOptions) ([]clicore.LocalMatch, error) {
		return []clicore.LocalMatch{{SessionID: "here", Peer: lanshare.ScannedPeer{Host: "192.168.1.9", Port: 7345, Fingerprint: "cert-fp"}}}, nil
	})
	local, cloud := routeLocally(t.Context(), []DeviceTarget{
		{SessionID: "here", LanFingerprint: guiFP},
		{SessionID: "away", LanFingerprint: guiFP2},
	})
	if len(local) != 1 || local[0].Target.SessionID != "here" {
		t.Fatalf("local = %+v, want just the device that answered", local)
	}
	if len(cloud) != 1 || cloud[0].SessionID != "away" {
		t.Fatalf("cloud = %+v, want just the device that did not", cloud)
	}
}

// Discovery failing must not strand a target. Everything falls back.
func TestADiscoveryErrorSendsEverythingToTheCloud(t *testing.T) {
	withMatch(t, func(context.Context, []clicore.DeviceRef, clicore.MatchOptions) ([]clicore.LocalMatch, error) {
		return nil, errors.New("network unreachable")
	})
	local, cloud := routeLocally(t.Context(), []DeviceTarget{{SessionID: "a", LanFingerprint: guiFP}})
	if len(local) != 0 || len(cloud) != 1 {
		t.Fatalf("local=%d cloud=%d, want everything in the cloud list", len(local), len(cloud))
	}
}

// Every target must come out of the split exactly once. A target dropped here is
// a file that silently never arrives, which is worse than an error.
func TestEveryTargetSurvivesTheSplitExactlyOnce(t *testing.T) {
	withMatch(t, func(context.Context, []clicore.DeviceRef, clicore.MatchOptions) ([]clicore.LocalMatch, error) {
		return []clicore.LocalMatch{{SessionID: "b", Peer: lanshare.ScannedPeer{Host: "10.0.0.2", Port: 7345}}}, nil
	})
	in := []DeviceTarget{
		{SessionID: "a", LanFingerprint: guiFP},
		{SessionID: "b", LanFingerprint: guiFP2},
		{SessionID: "c"}, // no fingerprint
	}
	local, cloud := routeLocally(t.Context(), in)

	seen := map[string]int{}
	for _, l := range local {
		seen[l.Target.SessionID]++
	}
	for _, c := range cloud {
		seen[c.SessionID]++
	}
	for _, want := range in {
		if seen[want.SessionID] != 1 {
			t.Errorf("target %q appeared %d times across the split, want exactly 1", want.SessionID, seen[want.SessionID])
		}
	}
}

// The pin is what makes a passwordless direct send safe: the scanned certificate
// is the one whose device card proved the identity.
func TestTheDirectSendPinsTheScannedCertificate(t *testing.T) {
	var got lanshare.SendOptions
	oldSend := lanSend
	lanSend = func(_ context.Context, _ string, _ int64, _ bool, _ io.Reader, o lanshare.SendOptions) (string, error) {
		got = o
		return "sha", nil
	}
	t.Cleanup(func() { lanSend = oldSend })

	dir := t.TempDir()
	path := dir + "/note.txt"
	if err := writeFileForTest(path, "hello"); err != nil {
		t.Fatalf("write: %v", err)
	}
	err := sendDirect(t.Context(), path,
		localTarget{Target: DeviceTarget{SessionID: "a"}, Peer: lanshare.ScannedPeer{Host: "10.0.0.2", Port: 7345, Fingerprint: "cert-fp"}}, nil)
	if err != nil {
		t.Fatalf("sendDirect: %v", err)
	}
	if got.PinFingerprint != "cert-fp" {
		t.Errorf("PinFingerprint = %q, want the scanned certificate fingerprint", got.PinFingerprint)
	}
	if got.Password != "" {
		t.Errorf("Password = %q, want empty: the pin authenticates this send", got.Password)
	}
	if got.Identity == nil {
		t.Error("no sender identity: the receiver cannot recognise this device")
	}
}

func writeFileForTest(path, body string) error {
	return os.WriteFile(path, []byte(body), 0o600)
}

// The cloud-cost prompt (P1-5). The owner's requirement is that a user must know
// cloud transfer and retention are the paid parts before they happen, so a
// silent upload is the thing being prevented.

func TestNoPromptWhenEverythingWentDirectly(t *testing.T) {
	withMatch(t, func(context.Context, []clicore.DeviceRef, clicore.MatchOptions) ([]clicore.LocalMatch, error) {
		return []clicore.LocalMatch{{SessionID: "a", Peer: lanshare.ScannedPeer{Host: "10.0.0.2", Port: 7345}}}, nil
	})
	_, cloud := routeLocally(t.Context(), []DeviceTarget{{SessionID: "a", LanFingerprint: guiFP}})
	if len(cloud) != 0 {
		t.Fatalf("cloud = %+v, want none", cloud)
	}
	// With nothing going to the cloud there is no cost to confirm. Asking anyway
	// is the nag that teaches people to dismiss the dialog unread.
}

// ONE upload covers every device, because the content key is sealed per device.
// A dialog that multiplied the size by the number of machines would overstate
// the cost and push people off a feature that is cheaper than they think.
func TestThePromptReportsTheFileSizeNotSizeTimesDevices(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/note.txt"
	if err := writeFileForTest(path, "0123456789"); err != nil {
		t.Fatalf("write: %v", err)
	}
	f := cloudFallbackFor(path, []DeviceTarget{
		{SessionID: "a", Name: "laptop"},
		{SessionID: "b", Name: "desktop"},
		{SessionID: "c", Name: "phone"},
	}, nil)
	if f.SizeBytes != 10 {
		t.Errorf("SizeBytes = %d, want 10 (one upload, not 30)", f.SizeBytes)
	}
	if len(f.DeviceNames) != 3 || f.DeviceNames[0] != "laptop" {
		t.Errorf("DeviceNames = %v, want the names the user picked", f.DeviceNames)
	}
}

// "Turn on discoverable" is only advice when one of the devices BEING UPLOADED
// FOR could have taken the file directly. For a device that can never be matched
// it is a suggestion nobody can act on.
func TestAdviceOnlyWhenADeviceBeingUploadedForCouldHaveTakenItDirectly(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/note.txt"
	if err := writeFileForTest(path, "x"); err != nil {
		t.Fatalf("write: %v", err)
	}
	if f := cloudFallbackFor(path, []DeviceTarget{{SessionID: "a", Name: "old"}}, nil); f.DirectWasPossible {
		t.Error("DirectWasPossible = true for a device with no fingerprint")
	}
	if f := cloudFallbackFor(path, []DeviceTarget{{SessionID: "a", Name: "laptop", LanFingerprint: guiFP}}, nil); !f.DirectWasPossible {
		t.Error("DirectWasPossible = false for a device that publishes a fingerprint")
	}
}

// A nil ConfirmCloud means "cannot ask", and must never block the send: the CLI,
// the tests and any headless caller are all in that state.
func TestANilConfirmDoesNotBlockTheSend(t *testing.T) {
	c := &Client{}
	if c.ConfirmCloud != nil {
		t.Fatal("ConfirmCloud should default to nil")
	}
}
