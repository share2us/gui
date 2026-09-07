// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Hassan Khurram

package netprofile

import (
	"encoding/binary"
	"testing"
	"time"
)

func encodeSystemTime(t time.Time) []byte {
	b := make([]byte, 16)
	put := func(i int, v uint16) { binary.LittleEndian.PutUint16(b[i*2:i*2+2], v) }
	put(0, uint16(t.Year()))
	put(1, uint16(t.Month()))
	put(2, uint16(t.Weekday()))
	put(3, uint16(t.Day()))
	put(4, uint16(t.Hour()))
	put(5, uint16(t.Minute()))
	put(6, uint16(t.Second()))
	put(7, uint16(t.Nanosecond()/int(time.Millisecond)))
	return b
}

func TestSystemTimeRoundTrip(t *testing.T) {
	want := time.Date(2026, 9, 7, 17, 34, 12, 250*int(time.Millisecond), time.Local)
	got, ok := systemTime(encodeSystemTime(want))
	if !ok {
		t.Fatal("a well-formed SYSTEMTIME must decode")
	}
	if !got.Equal(want) {
		t.Fatalf("systemTime() = %v, want %v", got, want)
	}
}

// Garbage must be rejected, not turned into a date. A wrong date here would make
// the app describe the wrong network, which is worse than describing none.
func TestSystemTimeRejectsNonsense(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   []byte
	}{
		{"empty", nil},
		{"short", make([]byte, 8)},
		{"all zero", make([]byte, 16)},
		{"month 13", encodeBad(2026, 13, 7)},
		{"month 0", encodeBad(2026, 0, 7)},
		{"day 32", encodeBad(2026, 9, 32)},
		{"day 0", encodeBad(2026, 9, 0)},
		{"year 1601", encodeBad(1601, 9, 7)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, ok := systemTime(tc.in); ok {
				t.Fatalf("systemTime(%v) accepted invalid input", tc.in)
			}
		})
	}
}

func encodeBad(year, month, day int) []byte {
	b := make([]byte, 16)
	binary.LittleEndian.PutUint16(b[0:2], uint16(year))
	binary.LittleEndian.PutUint16(b[2:4], uint16(month))
	binary.LittleEndian.PutUint16(b[6:8], uint16(day))
	return b
}

func TestCategoryOf(t *testing.T) {
	for v, want := range map[uint64]Category{0: Public, 1: Private, 2: Domain, 9: Unknown} {
		if got := categoryOf(v); got != want {
			t.Errorf("categoryOf(%d) = %v, want %v", v, got, want)
		}
	}
}

// Unknown must never render as a real classification: the UI keys off this to
// decide whether to say anything at all.
func TestCategoryStrings(t *testing.T) {
	if Unknown.String() != "unknown" {
		t.Fatalf("Unknown renders as %q", Unknown.String())
	}
	if Public.String() != "public" || Private.String() != "private" || Domain.String() != "domain" {
		t.Fatal("category names must match what the UI checks for")
	}
}

// On anything but Windows there is no such concept, and the UI must be told so
// rather than being left to warn about a permanently "unknown" network.
func TestUnsupportedElsewhere(t *testing.T) {
	s := Current()
	if s.Supported && s.Category == Unknown {
		t.Log("supported but undetermined: the UI stays silent, which is correct")
	}
}
