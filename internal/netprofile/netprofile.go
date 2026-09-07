// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Hassan Khurram

// Package netprofile reports how Windows has classified the network this machine
// is on.
//
// It matters because Windows Firewall rules are per-profile. A rule scoped to
// Private does not apply on a network Windows calls Public, and Windows calls
// plenty of ordinary home networks Public — so inbound discovery and transfers
// are dropped, with nothing on screen to say why. Two laptops on the same Wi-Fi
// simply never see each other.
//
// Everything here is best-effort by design. The honest states are Public,
// Private, Domain and Unknown, and Unknown is common: the caller must stay quiet
// rather than guess, because telling somebody their network is Public when it is
// not is exactly the kind of confident wrong statement this app has been fixing.
package netprofile

import (
	"encoding/binary"
	"time"
)

// Category is how the operating system classifies the current network.
type Category int

const (
	// Unknown means it could not be determined. Say nothing.
	Unknown Category = iota
	Public
	Private
	Domain
)

func (c Category) String() string {
	switch c {
	case Public:
		return "public"
	case Private:
		return "private"
	case Domain:
		return "domain"
	}
	return "unknown"
}

// Status describes the current network as far as it can be established.
type Status struct {
	Category Category `json:"category"`
	// Name is the network's profile name, when one was found, so a warning can
	// name the network the user is actually on.
	Name string `json:"name"`
	// Supported is false on platforms with no such concept, so the UI can leave
	// the subject alone entirely rather than reporting "unknown" forever.
	Supported bool `json:"supported"`
}

// Current reports the active network's classification.
func Current() Status { return current() }

// categoryOf maps the registry's Category value. Kept here, out of the
// Windows-only file, so the decoding can be tested on any machine — the parsing
// is where the mistakes live, and it needs no Windows to be wrong.
func categoryOf(v uint64) Category {
	switch v {
	case 0:
		return Public
	case 1:
		return Private
	case 2:
		return Domain
	}
	return Unknown
}

// systemTime decodes a Win32 SYSTEMTIME: eight little-endian uint16 fields,
// year, month, day-of-week, day, hour, minute, second, millisecond, in local
// time. Anything else is rejected rather than guessed at.
func systemTime(b []byte) (time.Time, bool) {
	if len(b) < 16 {
		return time.Time{}, false
	}
	f := make([]uint16, 8)
	for i := range f {
		f[i] = binary.LittleEndian.Uint16(b[i*2 : i*2+2])
	}
	year, month, day := int(f[0]), int(f[1]), int(f[3])
	if year < 1980 || month < 1 || month > 12 || day < 1 || day > 31 {
		return time.Time{}, false
	}
	return time.Date(year, time.Month(month), day, int(f[4]), int(f[5]), int(f[6]),
		int(f[7])*int(time.Millisecond), time.Local), true
}
