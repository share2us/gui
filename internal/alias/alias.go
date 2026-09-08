// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Hassan Khurram

// Package alias stores the names YOU give other people's devices.
//
// A device publishes a name it chose for itself, and that name is signed, so it
// cannot be taken by an impostor — but it is still whatever its owner typed.
// Three laptops called "DESKTOP-4F2K9A", or a housemate's phone called "laptop",
// are not a security problem and are still unusable in a list. This is the
// override: purely local, never transmitted, and never shown to anyone else.
//
// It is keyed on the device's STABLE identity fingerprint, which is the only
// thing here that survives what actually changes. An IP moves with DHCP; a MAC
// is invisible past the first router (so it is unavailable for every tailnet
// peer and across subnets, which is exactly where discovery needed help) and is
// randomised per-network by default on current systems; the per-session TLS
// certificate is regenerated every time the device restarts. The identity key is
// the one value that is stable, remote-unforgeable, and available before any
// transfer starts.
package alias

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// maxLen bounds an alias. It is a label in a narrow list, not a note.
const maxLen = 64

var mu sync.Mutex

type store struct {
	// Names maps identity fingerprint -> the name this user chose.
	Names map[string]string `json:"names"`
}

func path() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "share2us")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return filepath.Join(dir, "device-aliases.json"), nil
}

func load() store {
	s := store{Names: map[string]string{}}
	p, err := path()
	if err != nil {
		return s
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return s
		}
		return s
	}
	var got store
	if json.Unmarshal(raw, &got) != nil || got.Names == nil {
		return s // a corrupt file means no aliases, never a failure to list devices
	}
	return got
}

func save(s store) error {
	p, err := path()
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
	return os.Rename(tmp, p)
}

// All returns every alias, keyed by identity fingerprint.
func All() map[string]string {
	mu.Lock()
	defer mu.Unlock()
	out := map[string]string{}
	for k, v := range load().Names {
		out[k] = v
	}
	return out
}

// Set names a device, or clears the name when alias is empty. A device with no
// stable identity cannot be named: there would be nothing to attach the name to
// that survives the device restarting.
func Set(fingerprint, name string) error {
	fingerprint = strings.TrimSpace(fingerprint)
	if fingerprint == "" {
		return errors.New("this device has no stable identity to name")
	}
	name = clean(name)
	mu.Lock()
	defer mu.Unlock()
	s := load()
	if name == "" {
		delete(s.Names, fingerprint)
	} else {
		s.Names[fingerprint] = name
	}
	return save(s)
}

// clean bounds an alias and strips the control and bidi characters that would
// let a name blank a line or render as another device. The user types this one,
// so it is not an attack surface the way a published name is — but it lands in
// the same list, and a name pasted from somewhere else is not always typed.
func clean(s string) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(s) {
		switch {
		case r < 0x20, r == 0x7f:
			continue
		case r >= 0x80 && r <= 0x9f:
			continue
		case r >= 0x202a && r <= 0x202e, r >= 0x2066 && r <= 0x2069:
			continue
		}
		if b.Len() >= maxLen {
			break
		}
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}
