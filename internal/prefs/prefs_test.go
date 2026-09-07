// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Hassan Khurram

package prefs

import (
	"os"
	"path/filepath"
	"testing"
)

// isolate points the config directory at a temp dir so a test never reads or
// writes the developer's real preferences.
func isolate(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir) // macOS/Windows resolve UserConfigDir from the home dir
	t.Setenv("AppData", dir)
	return dir
}

// The bug: discoverability lived in memory, so every launch started invisible
// however the user had left it, and the app looked broken rather than off.
func TestDiscoverableSurvivesAReload(t *testing.T) {
	isolate(t)
	if Load().Discoverable {
		t.Fatal("a fresh install must not be discoverable")
	}
	if err := SetDiscoverable(true); err != nil {
		t.Fatalf("set: %v", err)
	}
	if !Load().Discoverable {
		t.Fatal("the choice must survive, or the next launch silently undoes it")
	}
	if err := SetDiscoverable(false); err != nil {
		t.Fatalf("unset: %v", err)
	}
	if Load().Discoverable {
		t.Fatal("turning it off must survive too")
	}
}

// Off is the safe default, so anything unreadable must land there rather than
// making a device discoverable it never agreed to.
func TestCorruptFileFallsBackToOff(t *testing.T) {
	dir := isolate(t)
	if err := SetDiscoverable(true); err != nil {
		t.Fatalf("set: %v", err)
	}
	f := filepath.Join(dir, "share2us", "gui-prefs.json")
	if err := os.WriteFile(f, []byte("{not json"), 0o600); err != nil {
		t.Fatalf("corrupt: %v", err)
	}
	if Load().Discoverable {
		t.Fatal("a corrupt file must never leave a device advertising itself")
	}
}

// The file holds a user's network posture, so it must not be world-readable.
func TestPrefsFileIsNotWorldReadable(t *testing.T) {
	dir := isolate(t)
	if err := SetDiscoverable(true); err != nil {
		t.Fatalf("set: %v", err)
	}
	fi, err := os.Stat(filepath.Join(dir, "share2us", "gui-prefs.json"))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := fi.Mode().Perm(); perm&0o077 != 0 {
		t.Fatalf("prefs file is %v, want no group/other access", perm)
	}
}

// A missing file is the first-run case and must not be an error path.
func TestMissingFileLoadsCleanDefaults(t *testing.T) {
	isolate(t)
	if Load().Discoverable {
		t.Fatal("no file means not discoverable")
	}
}
