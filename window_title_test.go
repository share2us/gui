// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Hassan Khurram

package main

import "testing"

// The address is in the title bar so it is readable from the OTHER laptop's
// point of view — you look at this machine's window (or its taskbar entry) to
// know which entry in the other machine's device list is this one. It was only
// in the status strip before, and only while discoverable, so the one moment it
// was needed was a moment it might not be shown.
func TestWindowTitle(t *testing.T) {
	cases := []struct {
		name  string
		addrs []string
		want  string
	}{
		{"lan address is shown", []string{"192.168.15.114"}, "Share2Us — 192.168.15.114"},
		{"the private address wins, since LocalAddresses sorts it first",
			[]string{"192.168.15.114", "100.64.0.9"}, "Share2Us — 192.168.15.114"},
		{"no address: the plain name, not a dangling separator", nil, "Share2Us"},
		{"an empty first entry is not an address either", []string{""}, "Share2Us"},
	}
	for _, c := range cases {
		if got := windowTitle(c.addrs); got != c.want {
			t.Errorf("%s: windowTitle(%v) = %q, want %q", c.name, c.addrs, got, c.want)
		}
	}
}
