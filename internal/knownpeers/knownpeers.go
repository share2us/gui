// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Hassan Khurram

// Package knownpeers remembers which certificate a nearby device presented last
// time, so a change can be noticed.
//
// It exists because of W-M4. The fingerprint a device advertises over mDNS is
// attacker-choosable — anyone on the network can announce the same display name
// with their own certificate — and the sender then pins exactly that, so the TLS
// session authenticates the impostor rather than the device. The 6-digit verify
// code is the defence, but only if somebody compares it, and asking on every
// send trains people to click through.
//
// This is the SSH known-hosts shape instead: quiet when nothing changed, loud
// when a familiar name turns up with a different key. First sight is still
// trust-on-first-use, which no local mechanism can fix; what it does buy is that
// an impersonation attempt against a device you have used before is impossible to
// miss.
//
// IT GRANTS NOTHING. That distinction matters: ADR-034 requires trust to be
// server-signed and MFA-gated, and lanid.Trust refuses to grant it locally. This
// is an observation cache, not a trust store. Being in it means "you have sent to
// this name before", never "this device may skip a check".
package knownpeers

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Status is what the store can say about a name/fingerprint pair.
type Status string

const (
	// New means this display name has not been sent to before. Show the verify
	// code once and let the user confirm.
	New Status = "new"
	// Same means the name presented the fingerprint it presented last time.
	// Nothing to say.
	Same Status = "same"
	// Changed means a name we know now presents a DIFFERENT certificate. Either
	// the device was reinstalled, or somebody is impersonating it. Say so.
	Changed Status = "changed"
	// Unknown means the peer advertised no fingerprint, so there is nothing to
	// compare and nothing to remember.
	Unknown Status = "unknown"
)

type peer struct {
	Fingerprint string    `json:"fingerprint"`
	FirstSeen   time.Time `json:"firstSeen"`
	LastSeen    time.Time `json:"lastSeen"`
}

type store struct {
	Peers map[string]peer `json:"peers"`
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
	return filepath.Join(dir, "known-peers.json"), nil
}

func load() store {
	s := store{Peers: map[string]peer{}}
	p, err := path()
	if err != nil {
		return s
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return store{Peers: map[string]peer{}}
		}
		return s
	}
	if err := json.Unmarshal(raw, &s); err != nil || s.Peers == nil {
		// A corrupt file must not make every device look like an impostor, which
		// is what returning Changed for everything would do. Start empty: the
		// worst case is one more first-sight confirmation per device.
		return store{Peers: map[string]peer{}}
	}
	return s
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

// key normalises the display name. Names come off the network, so they are
// compared case-insensitively and trimmed; an empty name is not storable.
func key(name string) string { return strings.ToLower(strings.TrimSpace(name)) }

func norm(fp string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(fp), ":", ""))
}

// Check reports what is known about this name/fingerprint pair, and the
// fingerprint previously seen when it has changed. It records nothing: the
// caller decides whether the user accepted, and calls Remember only then.
func Check(name, fingerprint string) (Status, string) {
	if key(name) == "" || norm(fingerprint) == "" {
		return Unknown, ""
	}
	mu.Lock()
	defer mu.Unlock()
	known, ok := load().Peers[key(name)]
	switch {
	case !ok:
		return New, ""
	case norm(known.Fingerprint) == norm(fingerprint):
		return Same, ""
	default:
		return Changed, known.Fingerprint
	}
}

// Remember records that the user accepted this name presenting this
// fingerprint. Call it AFTER a confirmation, never before: writing on sight
// would mean an impostor seen once is silently normal on the second attempt.
func Remember(name, fingerprint string) error {
	if key(name) == "" || norm(fingerprint) == "" {
		return nil
	}
	mu.Lock()
	defer mu.Unlock()
	s := load()
	now := time.Now()
	first := now
	if prev, ok := s.Peers[key(name)]; ok && norm(prev.Fingerprint) == norm(fingerprint) {
		first = prev.FirstSeen
	}
	s.Peers[key(name)] = peer{Fingerprint: fingerprint, FirstSeen: first, LastSeen: now}
	return save(s)
}

// Forget drops a name, so the next send is treated as first sight. This is the
// escape hatch for the honest case behind a Changed verdict: the device really
// was reinstalled.
func Forget(name string) error {
	if key(name) == "" {
		return nil
	}
	mu.Lock()
	defer mu.Unlock()
	s := load()
	delete(s.Peers, key(name))
	return save(s)
}
