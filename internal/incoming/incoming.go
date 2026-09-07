// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Hassan Khurram

// Package incoming holds files that have arrived until the user says where they
// go.
//
// Before this, a received file was written straight into a Downloads folder and
// announced with a toast: the user never chose, and afterwards had to go looking
// for it. Now an arrival waits here and only lands somewhere the user picked
// (owner decision, 2026-09-07).
//
// Staging is ON DISK rather than in memory for one reason that matters: closing
// the app must not lose a file somebody sent you. The index is written after the
// file, so a crash between the two leaves an orphan on disk rather than an index
// entry pointing at nothing.
package incoming

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"time"
)

// Item is one arrival waiting to be filed.
type Item struct {
	ID   string    `json:"id"`
	Name string    `json:"name"` // the sender's file name, shown to the user
	From string    `json:"from"` // sender device name, "" when anonymous
	Size int64     `json:"size"`
	At   time.Time `json:"at"`
	// File is the staged copy's absolute path. Never shown: the user cares about
	// Name, and this location is an implementation detail they did not choose.
	File string `json:"file"`
}

// store is the on-disk index plus the folder the user chose to save into.
type store struct {
	Items []Item `json:"items"`
	// Folder is the remembered destination. Empty means "ask me", which is the
	// default: nothing is written to a location the user never picked.
	Folder string `json:"folder"`
}

var mu sync.Mutex

// Dir is the staging directory, 0700 because it holds other people's files.
func Dir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	d := filepath.Join(base, "share2us", "incoming")
	if err := os.MkdirAll(d, 0o700); err != nil {
		return "", err
	}
	return d, nil
}

func indexPath() (string, error) {
	d, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "index.json"), nil
}

func load() (store, error) {
	var s store
	p, err := indexPath()
	if err != nil {
		return s, err
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return s, nil // nothing has arrived yet
		}
		return s, err
	}
	if err := json.Unmarshal(raw, &s); err != nil {
		// A corrupt index must not hide the files themselves; start a fresh index
		// rather than refusing to run.
		return store{}, nil
	}
	return s, nil
}

func save(s store) error {
	p, err := indexPath()
	if err != nil {
		return err
	}
	raw, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, p) // atomic: a torn index would lose every arrival
}

// StagePath returns where a receiver should write an arrival, before it is known
// what the file will be called on disk. Callers hand the result to the transfer
// as its destination directory.
func StagePath() (string, error) { return Dir() }

// Add records a file that has landed in the staging directory.
func Add(name, from, file string, size int64) (Item, error) {
	mu.Lock()
	defer mu.Unlock()
	s, err := load()
	if err != nil {
		return Item{}, err
	}
	it := Item{
		ID:   strconv.FormatInt(time.Now().UnixNano(), 36),
		Name: name,
		From: from,
		Size: size,
		At:   time.Now(),
		File: file,
	}
	s.Items = append(s.Items, it)
	return it, save(s)
}

// List returns what is waiting, newest first.
func List() []Item {
	mu.Lock()
	defer mu.Unlock()
	s, _ := load()
	out := make([]Item, 0, len(s.Items))
	for _, it := range s.Items {
		if _, err := os.Stat(it.File); err == nil {
			out = append(out, it) // an entry whose file vanished is not waiting
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].At.After(out[j].At) })
	return out
}

// Get returns one item.
func Get(id string) (Item, bool) {
	for _, it := range List() {
		if it.ID == id {
			return it, true
		}
	}
	return Item{}, false
}

// Folder is the remembered save location, or "" for "ask every time".
func Folder() string {
	mu.Lock()
	defer mu.Unlock()
	s, _ := load()
	return s.Folder
}

// SetFolder remembers where arrivals should go from now on. Passing "" restores
// asking each time, so the choice is never a trap.
func SetFolder(dir string) error {
	mu.Lock()
	defer mu.Unlock()
	s, err := load()
	if err != nil {
		return err
	}
	s.Folder = dir
	return save(s)
}

// Claim moves a staged file to dest and drops it from the index. The staged copy
// is only forgotten once the move has succeeded, so a failure leaves the file
// still waiting rather than losing it.
func Claim(id, dest string) error {
	it, ok := Get(id)
	if !ok {
		return fmt.Errorf("incoming: %s is no longer waiting", id)
	}
	if err := move(it.File, dest); err != nil {
		return err
	}
	return forget(id)
}

// Discard deletes a staged file the user does not want.
func Discard(id string) error {
	it, ok := Get(id)
	if !ok {
		return nil
	}
	_ = os.Remove(it.File)
	return forget(id)
}

func forget(id string) error {
	mu.Lock()
	defer mu.Unlock()
	s, err := load()
	if err != nil {
		return err
	}
	kept := s.Items[:0]
	for _, it := range s.Items {
		if it.ID != id {
			kept = append(kept, it)
		}
	}
	s.Items = kept
	return save(s)
}

// move renames where it can and falls back to copy+delete across filesystems,
// which is the common case here: staging is under the user's config directory
// and the destination is wherever they chose.
func move(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := out.ReadFrom(in); err != nil {
		out.Close()
		os.Remove(dst) // do not leave a half-written file where the user asked
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	in.Close()
	return os.Remove(src)
}

// Sweep deletes arrivals nobody filed within maxAge and reports how many went.
// A file nobody saves must not accumulate forever; the caller is expected to
// tell the user the count rather than let files disappear silently.
func Sweep(maxAge time.Duration) int {
	cutoff := time.Now().Add(-maxAge)
	n := 0
	for _, it := range List() {
		if it.At.Before(cutoff) {
			if Discard(it.ID) == nil {
				n++
			}
		}
	}
	return n
}
