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

// ---- §AG D1: one arrival path, whatever brought the file in ----------------

// With no folder chosen, an arrival waits. This is the 2026-09-07 rule ("the
// user never chose, and afterwards had to go looking for it") — cloud
// device-sends now obey it too, where before they were written straight into
// the Downloads folder.
func TestDeliverStagesWhenNoFolderIsChosen(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	src := filepath.Join(t.TempDir(), "report.pdf")
	if err := os.WriteFile(src, []byte("bytes"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := Deliver("report.pdf", "openclaw", src, 5)
	if err != nil {
		t.Fatal(err)
	}
	if got.Filed {
		t.Fatal("nothing was chosen, so nothing may be filed")
	}
	if _, err := os.Stat(got.Path); err != nil {
		t.Fatalf("the staged file is not there: %v", err)
	}
	if len(List()) != 1 {
		t.Fatalf("the arrival is not waiting: %+v", List())
	}
}

// A remembered folder means "stop asking me": the file goes there and there is
// nothing left to decide.
func TestDeliverFilesIntoTheChosenFolder(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	folder := t.TempDir()
	if err := SetFolder(folder); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(t.TempDir(), "report.pdf")
	if err := os.WriteFile(src, []byte("bytes"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := Deliver("report.pdf", "openclaw", src, 5)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Filed {
		t.Fatal("a remembered folder must take the file")
	}
	if want := filepath.Join(folder, "report.pdf"); got.Path != want {
		t.Fatalf("path = %q, want %q", got.Path, want)
	}
	if _, err := os.Stat(got.Path); err != nil {
		t.Fatalf("the filed file is not there: %v", err)
	}
}

// Filing over an existing name destroyed the earlier file with no prompt and no
// record. A second arrival of the same name must not overwrite the first.
func TestDeliverDoesNotOverwriteAnExistingFile(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	folder := t.TempDir()
	if err := SetFolder(folder); err != nil {
		t.Fatal(err)
	}
	existing := filepath.Join(folder, "report.pdf")
	if err := os.WriteFile(existing, []byte("the first one"), 0o600); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(t.TempDir(), "report.pdf")
	if err := os.WriteFile(src, []byte("the second one"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := Deliver("report.pdf", "openclaw", src, 14)
	if err != nil {
		t.Fatal(err)
	}
	if got.Path == existing {
		t.Fatal("the earlier file was overwritten")
	}
	first, _ := os.ReadFile(existing)
	if string(first) != "the first one" {
		t.Fatalf("the earlier file was modified: %q", first)
	}
}

// A folder that cannot be written to (unplugged drive, revoked permission) must
// never lose the file: it falls back to staging.
func TestDeliverFallsBackToStagingWhenTheFolderIsUnusable(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := SetFolder(filepath.Join(t.TempDir(), "does", "not", "exist")); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(t.TempDir(), "report.pdf")
	if err := os.WriteFile(src, []byte("bytes"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := Deliver("report.pdf", "openclaw", src, 5)
	if err != nil {
		t.Fatal(err)
	}
	if got.Filed {
		t.Fatal("an unusable folder must not report the file as filed")
	}
	if _, err := os.Stat(got.Path); err != nil {
		t.Fatalf("the file was lost: %v", err)
	}
}
