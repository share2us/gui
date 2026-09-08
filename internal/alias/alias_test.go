// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Hassan Khurram

package alias

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func isolate(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	return dir
}

func TestSetAndClear(t *testing.T) {
	isolate(t)
	if err := Set("ID1", "Study laptop"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if got := All()["ID1"]; got != "Study laptop" {
		t.Fatalf("got %q, want Study laptop", got)
	}
	// Empty clears rather than storing a blank name, so the device's own name
	// comes back instead of the row going nameless.
	if err := Set("ID1", "  "); err != nil {
		t.Fatalf("clear: %v", err)
	}
	if _, ok := All()["ID1"]; ok {
		t.Error("an emptied alias should be removed, not stored blank")
	}
}

func TestAliasSurvivesAcrossLoads(t *testing.T) {
	dir := isolate(t)
	if err := Set("ID1", "Study laptop"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "share2us", "device-aliases.json")); err != nil {
		t.Fatalf("nothing was persisted: %v", err)
	}
	if All()["ID1"] != "Study laptop" {
		t.Error("alias did not survive a reload")
	}
}

// A device with no card has no stable identity. Naming it would mean keying on
// its address, which moves to whatever holds that address next.
func TestADeviceWithNoIdentityCannotBeNamed(t *testing.T) {
	isolate(t)
	if err := Set("", "Study laptop"); err == nil {
		t.Fatal("naming a device with no identity should be refused, not silently kept")
	}
}

func TestNameIsBoundedAndCleaned(t *testing.T) {
	isolate(t)
	if err := Set("ID1", strings.Repeat("n", maxLen*3)); err != nil {
		t.Fatal(err)
	}
	if len(All()["ID1"]) > maxLen {
		t.Errorf("alias is %d bytes, want at most %d", len(All()["ID1"]), maxLen)
	}
	if err := Set("ID2", "study\rlaptop‮"); err != nil {
		t.Fatal(err)
	}
	got := All()["ID2"]
	if strings.ContainsAny(got, "\r\n") || strings.ContainsRune(got, 0x202e) {
		t.Errorf("alias %q still carries control characters", got)
	}
}

// A corrupt file must not be able to stop the device list from being drawn.
func TestACorruptStoreMeansNoAliasesNotAFailure(t *testing.T) {
	dir := isolate(t)
	sub := filepath.Join(dir, "share2us")
	if err := os.MkdirAll(sub, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "device-aliases.json"), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := All(); len(got) != 0 {
		t.Fatalf("got %v, want an empty set", got)
	}
	if err := Set("ID1", "Study laptop"); err != nil {
		t.Fatalf("a corrupt store should be replaced, not fatal: %v", err)
	}
	if All()["ID1"] != "Study laptop" {
		t.Error("could not recover from a corrupt store")
	}
}
