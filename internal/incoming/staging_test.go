// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Hassan Khurram

package incoming

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func isolate(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	t.Setenv("AppData", dir)
	return dir
}

// write a file where a receiver would have put it, under the sender's own name.
func arrive(t *testing.T, name string) string {
	t.Helper()
	d, err := Dir()
	if err != nil {
		t.Fatalf("dir: %v", err)
	}
	p := filepath.Join(d, name)
	if err := os.WriteFile(p, []byte(name), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	return p
}

// The bug: the receiver writes every arrival under the sender's file name into
// one directory and refuses a transfer whose name is already there. A second
// "report.pdf" failed the whole transfer while the first still waited.
func TestStageFreesTheSenderName(t *testing.T) {
	isolate(t)
	d, _ := Dir()

	first := arrive(t, "report.pdf")
	if _, err := Stage("report.pdf", "kestrel", first, 4); err != nil {
		t.Fatalf("stage first: %v", err)
	}
	if _, err := os.Stat(filepath.Join(d, "report.pdf")); !os.IsNotExist(err) {
		t.Fatal("the sender's name must be free for the next transfer")
	}

	second := arrive(t, "report.pdf") // the next transfer can now land
	if _, err := Stage("report.pdf", "kestrel", second, 4); err != nil {
		t.Fatalf("stage second: %v", err)
	}

	items := List()
	if len(items) != 2 {
		t.Fatalf("got %d waiting, want 2 — both arrivals must survive", len(items))
	}
	if items[0].File == items[1].File {
		t.Fatal("two arrivals share one path; one has overwritten the other")
	}
	for _, it := range items {
		if it.Name != "report.pdf" {
			t.Errorf("the user-facing name must stay the sender's: %q", it.Name)
		}
		if _, err := os.Stat(it.File); err != nil {
			t.Errorf("staged file missing: %v", err)
		}
	}
}

// A filed arrival is the user's own file in the folder they chose. Retention may
// stop listing it; it must never delete it.
func TestSweepNeverDeletesAFiledArrival(t *testing.T) {
	dir := isolate(t)
	folder := filepath.Join(dir, "Downloads")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	saved := filepath.Join(folder, "report.pdf")
	if err := os.WriteFile(saved, []byte("mine"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	it, err := AddFiled("report.pdf", "kestrel", saved, 4, folder)
	if err != nil {
		t.Fatalf("addfiled: %v", err)
	}
	if len(List()) != 1 {
		t.Fatal("a filed arrival must still be listed, so a one-off can be redirected")
	}

	// Age it past the window.
	backdate(t, it.ID, time.Now().Add(-48*time.Hour))
	if n := Sweep(24 * time.Hour); n != 0 {
		t.Fatalf("Sweep reported %d deletions; a filed arrival is not a deletion", n)
	}
	if len(List()) != 0 {
		t.Fatal("an expired filed arrival should stop being listed")
	}
	if _, err := os.Stat(saved); err != nil {
		t.Fatalf("THE USER'S FILE WAS DELETED: %v", err)
	}
}

// An unfiled arrival is a staged copy nobody claimed; that one does get removed.
func TestSweepDeletesUnclaimedStaging(t *testing.T) {
	isolate(t)
	it, err := Stage("report.pdf", "kestrel", arrive(t, "report.pdf"), 4)
	if err != nil {
		t.Fatalf("stage: %v", err)
	}
	backdate(t, it.ID, time.Now().Add(-48*time.Hour))
	if n := Sweep(24 * time.Hour); n != 1 {
		t.Fatalf("Sweep = %d, want 1", n)
	}
	if _, err := os.Stat(it.File); !os.IsNotExist(err) {
		t.Fatal("an expired staged copy should be gone")
	}
}

// backdate rewrites one item's arrival time so retention can be tested without
// waiting.
func backdate(t *testing.T, id string, when time.Time) {
	t.Helper()
	mu.Lock()
	defer mu.Unlock()
	s, err := load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	for i := range s.Items {
		if s.Items[i].ID == id {
			s.Items[i].At = when
		}
	}
	if err := save(s); err != nil {
		t.Fatalf("save: %v", err)
	}
}
