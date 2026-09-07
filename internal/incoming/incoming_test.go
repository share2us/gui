// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Hassan Khurram

package incoming

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// stage puts a real file in the staging dir and records it, the way a receiver
// would.
func stage(t *testing.T, name, body string) Item {
	t.Helper()
	d, err := Dir()
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(d, name)
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	it, err := Add(name, "laptop", p, int64(len(body)))
	if err != nil {
		t.Fatal(err)
	}
	return it
}

func TestClaimMovesTheFileAndStopsWaiting(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	it := stage(t, "report.pdf", "hello")

	if got := List(); len(got) != 1 || got[0].Name != "report.pdf" {
		t.Fatalf("expected one waiting arrival, got %+v", got)
	}

	dest := filepath.Join(t.TempDir(), "saved.pdf")
	if err := Claim(it.ID, dest); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(dest)
	if err != nil || string(b) != "hello" {
		t.Fatalf("file did not arrive intact at the chosen path: %v %q", err, b)
	}
	if len(List()) != 0 {
		t.Error("a claimed file should stop being listed as waiting")
	}
	if _, err := os.Stat(it.File); !os.IsNotExist(err) {
		t.Error("the staged copy should be gone once it has been filed")
	}
}

// The staged copy must survive a failed save, or a mistyped path loses someone
// else's file.
func TestFailedClaimKeepsTheFileWaiting(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	it := stage(t, "keep.bin", "data")

	err := Claim(it.ID, filepath.Join(t.TempDir(), "no-such-dir", "keep.bin"))
	if err == nil {
		t.Fatal("saving into a missing directory should fail")
	}
	if len(List()) != 1 {
		t.Fatal("the file must still be waiting after a failed save")
	}
	if _, serr := os.Stat(it.File); serr != nil {
		t.Error("the staged copy must still exist after a failed save")
	}
}

func TestFolderRoundTripAndAskAgain(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if Folder() != "" {
		t.Error("default must be ask-every-time, not a folder the user never chose")
	}
	if err := SetFolder("/tmp/somewhere"); err != nil {
		t.Fatal(err)
	}
	if Folder() != "/tmp/somewhere" {
		t.Errorf("Folder() = %q", Folder())
	}
	if err := SetFolder(""); err != nil {
		t.Fatal(err)
	}
	if Folder() != "" {
		t.Error("clearing the folder must restore asking each time")
	}
}

func TestSweepRemovesOnlyStaleArrivals(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	fresh := stage(t, "fresh.txt", "new")
	old := stage(t, "old.txt", "old")

	// Backdate one arrival by rewriting the index the way time would have.
	s, _ := load()
	for i := range s.Items {
		if s.Items[i].ID == old.ID {
			s.Items[i].At = time.Now().Add(-8 * 24 * time.Hour)
		}
	}
	if err := save(s); err != nil {
		t.Fatal(err)
	}

	if n := Sweep(7 * 24 * time.Hour); n != 1 {
		t.Fatalf("Sweep removed %d, want 1", n)
	}
	left := List()
	if len(left) != 1 || left[0].ID != fresh.ID {
		t.Fatalf("sweep took the wrong file: %+v", left)
	}
}

// An index entry whose file has been deleted underneath us is not "waiting".
func TestListSkipsEntriesWhoseFileIsGone(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	it := stage(t, "vanish.txt", "x")
	if err := os.Remove(it.File); err != nil {
		t.Fatal(err)
	}
	if got := List(); len(got) != 0 {
		t.Errorf("expected nothing waiting, got %+v", got)
	}
}
