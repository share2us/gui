// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Hassan Khurram

package knownpeers

import (
	"os"
	"path/filepath"
	"testing"
)

func isolate(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	t.Setenv("AppData", dir)
	return dir
}

const fpA = "aa:bb:cc:dd:ee:ff"
const fpB = "11:22:33:44:55:66"

// The whole point: quiet when nothing changed, loud when a familiar name turns
// up with a different certificate.
func TestLifecycle(t *testing.T) {
	isolate(t)

	if st, _ := Check("kestrel", fpA); st != New {
		t.Fatalf("first sight = %v, want New", st)
	}
	// Nothing is recorded until the user accepts.
	if st, _ := Check("kestrel", fpA); st != New {
		t.Fatalf("Check must not record; got %v on a second look", st)
	}

	if err := Remember("kestrel", fpA); err != nil {
		t.Fatalf("remember: %v", err)
	}
	if st, _ := Check("kestrel", fpA); st != Same {
		t.Fatalf("after accepting, = %v, want Same", st)
	}

	st, prev := Check("kestrel", fpB)
	if st != Changed {
		t.Fatalf("a new certificate for a known name = %v, want Changed", st)
	}
	if prev != fpA {
		t.Errorf("previous fingerprint = %q, want the one accepted before", prev)
	}
}

// An impostor seen once must not become normal on its second attempt. That is
// why Check never writes.
func TestAnImpostorDoesNotBecomeFamiliarByShowingUpTwice(t *testing.T) {
	isolate(t)
	Remember("kestrel", fpA)
	for i := 0; i < 3; i++ {
		if st, _ := Check("kestrel", fpB); st != Changed {
			t.Fatalf("attempt %d = %v, want Changed every time", i+1, st)
		}
	}
}

// Names arrive off the network; compare them the way a person reads them.
func TestNameAndFingerprintComparisonIsForgiving(t *testing.T) {
	isolate(t)
	Remember("Kestrel", fpA)
	for _, name := range []string{"kestrel", "KESTREL", "  kestrel  "} {
		if st, _ := Check(name, fpA); st != Same {
			t.Errorf("name %q = %v, want Same", name, st)
		}
	}
	// The same key with and without separators is the same key.
	if st, _ := Check("kestrel", "AABBCCDDEEFF"); st != Same {
		t.Errorf("colon-free fingerprint = %v, want Same", st)
	}
}

// A peer that advertised no fingerprint gives nothing to compare, and must not
// be stored — storing an empty key would make the next real one look Changed.
func TestNoFingerprintIsUnknownAndNotStored(t *testing.T) {
	isolate(t)
	if st, _ := Check("kestrel", ""); st != Unknown {
		t.Fatal("a peer with no fingerprint is Unknown")
	}
	if err := Remember("kestrel", ""); err != nil {
		t.Fatalf("remember: %v", err)
	}
	if st, _ := Check("kestrel", fpA); st != New {
		t.Fatal("an empty fingerprint must not have been recorded")
	}
}

// The honest case behind a Changed verdict is a reinstalled device.
func TestForgetRestoresFirstSight(t *testing.T) {
	isolate(t)
	Remember("kestrel", fpA)
	if err := Forget("kestrel"); err != nil {
		t.Fatalf("forget: %v", err)
	}
	if st, _ := Check("kestrel", fpB); st != New {
		t.Fatalf("after forgetting = %v, want New", st)
	}
}

// A corrupt file must not make every device look like an impostor: that would
// train people to click through the one warning that matters.
func TestCorruptStoreFallsBackToFirstSight(t *testing.T) {
	dir := isolate(t)
	Remember("kestrel", fpA)
	if err := os.WriteFile(filepath.Join(dir, "share2us", "known-peers.json"), []byte("{not json"), 0o600); err != nil {
		t.Fatalf("corrupt: %v", err)
	}
	if st, _ := Check("kestrel", fpB); st != New {
		t.Fatalf("corrupt store = %v, want New rather than Changed", st)
	}
}

// It holds other people's device names; it should not be world-readable.
func TestStoreIsNotWorldReadable(t *testing.T) {
	dir := isolate(t)
	Remember("kestrel", fpA)
	fi, err := os.Stat(filepath.Join(dir, "share2us", "known-peers.json"))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := fi.Mode().Perm(); perm&0o077 != 0 {
		t.Fatalf("mode %v, want no group or other access", perm)
	}
}
