// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Hassan Khurram

// Package prefs stores the handful of desktop-app choices that must survive a
// restart.
//
// It exists because discoverability did not. It was an in-memory field, so every
// launch began with the app invisible no matter what the user had set, and the
// only way to notice was to close the app and look again. That is the most
// likely explanation for a "nearby share is not working" report: the setting was
// switched on once and silently forgotten.
package prefs

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

// Prefs is the persisted set. Deliberately small: anything that belongs to the
// CLI as well lives in the shared config, not here.
type Prefs struct {
	// Discoverable is whether this device advertises itself and accepts inbound
	// transfers. Off unless the user has said otherwise, and remembered once they
	// have.
	Discoverable bool `json:"discoverable"`
}

var mu sync.Mutex

func path() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "share2us")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return filepath.Join(dir, "gui-prefs.json"), nil
}

// Load returns the stored preferences. A missing or unreadable file yields the
// zero value, which is the safe default: not discoverable.
func Load() Prefs {
	mu.Lock()
	defer mu.Unlock()
	var p Prefs
	f, err := path()
	if err != nil {
		return p
	}
	raw, err := os.ReadFile(f)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return Prefs{}
		}
		return p
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return Prefs{} // corrupt file: fall back to off rather than guessing
	}
	return p
}

// SetDiscoverable records the choice so the next launch honours it.
func SetDiscoverable(on bool) error {
	mu.Lock()
	defer mu.Unlock()
	p := Prefs{}
	f, err := path()
	if err != nil {
		return err
	}
	if raw, rerr := os.ReadFile(f); rerr == nil {
		_ = json.Unmarshal(raw, &p)
	}
	p.Discoverable = on
	raw, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	tmp := f + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, f)
}
