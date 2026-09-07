// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Hassan Khurram

//go:build windows

package netprofile

import (
	"time"

	"golang.org/x/sys/windows/registry"
)

// Windows records one key per network it has seen under NetworkList\Profiles,
// each carrying a Category (0 public, 1 private, 2 domain) and the time it was
// last connected. There is no registry value naming "the current network", so
// the most recently connected profile is the one in use.
//
// Read from the registry rather than by running Get-NetConnectionProfile: this
// app already worried a user by opening a console window on a timer, and a
// desktop app spawning powershell.exe is a heuristic antivirus software reacts
// to. A registry read starts no process at all.
const profilesKey = `SOFTWARE\Microsoft\Windows NT\CurrentVersion\NetworkList\Profiles`

// staleAfter bounds how old a "last connected" stamp may be and still describe
// the current network. A machine that has been on this network for a week still
// has a recent stamp, because Windows updates it while connected; a stamp older
// than this is a network we are no longer on, and reporting its category would
// be worse than reporting nothing.
const staleAfter = 24 * time.Hour

func current() Status {
	root, err := registry.OpenKey(registry.LOCAL_MACHINE, profilesKey, registry.ENUMERATE_SUB_KEYS|registry.QUERY_VALUE)
	if err != nil {
		return Status{Supported: true} // the concept exists here; the answer does not
	}
	defer root.Close()

	names, err := root.ReadSubKeyNames(-1)
	if err != nil {
		return Status{Supported: true}
	}

	var (
		best     time.Time
		bestCat  Category
		bestName string
		// ties records whether two profiles claim the same most-recent moment,
		// which happens with several adapters connected at once. Then there is no
		// single "current network" and the honest answer is Unknown.
		ties bool
	)
	for _, name := range names {
		k, err := registry.OpenKey(root, name, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		cat, _, cerr := k.GetIntegerValue("Category")
		raw, _, derr := k.GetBinaryValue("DateLastConnected")
		profileName, _, _ := k.GetStringValue("ProfileName")
		k.Close()
		if cerr != nil || derr != nil {
			continue
		}
		when, ok := systemTime(raw)
		if !ok {
			continue
		}
		switch {
		case when.After(best):
			best, bestCat, bestName, ties = when, categoryOf(cat), profileName, false
		case when.Equal(best):
			// Same instant, different category: no basis to choose between them.
			if categoryOf(cat) != bestCat {
				ties = true
			}
		}
	}
	if best.IsZero() || ties || time.Since(best) > staleAfter {
		return Status{Supported: true}
	}
	return Status{Category: bestCat, Name: bestName, Supported: true}
}
