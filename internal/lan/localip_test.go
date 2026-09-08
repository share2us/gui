// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Hassan Khurram

package lan

import (
	"net"
	"testing"
)

func ifc(name string, mac ...byte) net.Interface {
	return net.Interface{Name: name, HardwareAddr: mac}
}

// The report this exists for: a Windows machine with WSL/Hyper-V showed
// "This device 172.21.208.1" while the other laptop saw it as 192.168.10.218.
// Both are private, so they tied, and the tie broke on interface enumeration
// order — where Windows tends to list vEthernet first.
func TestARealLANAddressBeatsAHostOnlyOne(t *testing.T) {
	hyperv := localIPv4Score(net.ParseIP("172.21.208.1").To4(), ifc("vEthernet (WSL)", 0x00, 0x15, 0x5D, 0x01, 0x02, 0x03))
	lan := localIPv4Score(net.ParseIP("192.168.10.218").To4(), ifc("Wi-Fi", 0xAC, 0xDE, 0x48, 0x01, 0x02, 0x03))
	if !(lan > hyperv) {
		t.Fatalf("LAN scored %d, host-only scored %d; the real LAN address must win", lan, hyperv)
	}
}

func TestVirtualAdaptersAreRecognised(t *testing.T) {
	cases := []struct {
		name string
		i    net.Interface
		want bool
	}{
		// The MAC prefix is the signal that survives localisation: Windows returns
		// a translated friendly name, so name matching alone is not enough there.
		{"hyper-v by mac, name in another language", ifc("Netzwerkbrücke", 0x00, 0x15, 0x5D, 0, 0, 1), true},
		{"vmware by mac", ifc("Ethernet 2", 0x00, 0x50, 0x56, 0, 0, 1), true},
		{"virtualbox by mac", ifc("Ethernet 3", 0x08, 0x00, 0x27, 0, 0, 1), true},
		{"wsl by name", ifc("vEthernet (WSL)"), true},
		{"docker by name", ifc("docker0"), true},
		{"libvirt by name", ifc("virbr0"), true},
		{"a real wifi adapter", ifc("Wi-Fi", 0xAC, 0xDE, 0x48, 0, 0, 1), false},
		{"a real ethernet adapter", ifc("eth0", 0xAC, 0xDE, 0x48, 0, 0, 2), false},
		{"an adapter with no mac at all", ifc("Ethernet"), false},
	}
	for _, c := range cases {
		if got := virtualIface(c.i); got != c.want {
			t.Errorf("%s: virtualIface = %v, want %v", c.name, got, c.want)
		}
	}
}

// The order the existing scorer established, which this must not disturb.
func TestScoreOrderingIsUnchangedForEverythingElse(t *testing.T) {
	real := ifc("eth0", 0xAC, 0xDE, 0x48, 0, 0, 1)
	apipa := localIPv4Score(net.ParseIP("169.254.1.1").To4(), real)
	public := localIPv4Score(net.ParseIP("93.184.216.34").To4(), real)
	tailnet := localIPv4Score(net.ParseIP("100.100.25.200").To4(), real)
	priv := localIPv4Score(net.ParseIP("192.168.1.5").To4(), real)
	if !(priv > tailnet && tailnet > public && public > apipa) {
		t.Fatalf("ordering broke: private=%d tailnet=%d public=%d apipa=%d", priv, tailnet, public, apipa)
	}
}

// A host-only address is still worse than a tailnet one: the tailnet address is
// reachable by an actual peer, and 172.21.208.1 is reachable by nothing but this
// machine.
func TestAHostOnlyAddressIsTheLeastUsefulRoutableAnswer(t *testing.T) {
	hostOnly := localIPv4Score(net.ParseIP("172.21.208.1").To4(), ifc("vEthernet (WSL)"))
	apipa := localIPv4Score(net.ParseIP("169.254.1.1").To4(), ifc("eth0"))
	tailnet := localIPv4Score(net.ParseIP("100.100.25.200").To4(), ifc("eth0"))
	if !(tailnet > hostOnly) {
		t.Errorf("tailnet=%d must beat host-only=%d", tailnet, hostOnly)
	}
	if !(hostOnly > apipa) {
		t.Errorf("host-only=%d should still beat an unreachable APIPA=%d", hostOnly, apipa)
	}
}

// RankedIPv4s runs against whatever this machine has. It cannot assert a
// specific address, but it can assert the invariants every caller relies on.
func TestRankedIPv4sIsSortedAndExcludesLoopback(t *testing.T) {
	got := RankedIPv4s()
	for _, ip := range got {
		if net.ParseIP(ip).IsLoopback() {
			t.Errorf("loopback %s must never be offered as this device's address", ip)
		}
		if net.ParseIP(ip).To4() == nil {
			t.Errorf("%s is not IPv4", ip)
		}
	}
	if len(got) > 1 {
		ifaces, _ := net.Interfaces()
		byIP := map[string]net.Interface{}
		for _, i := range ifaces {
			addrs, _ := i.Addrs()
			for _, a := range addrs {
				if n, ok := a.(*net.IPNet); ok && n.IP.To4() != nil {
					byIP[n.IP.To4().String()] = i
				}
			}
		}
		prev := localIPv4Score(net.ParseIP(got[0]).To4(), byIP[got[0]])
		for _, ip := range got[1:] {
			s := localIPv4Score(net.ParseIP(ip).To4(), byIP[ip])
			if s > prev {
				t.Fatalf("%s (score %d) sorted after a lower-scoring address (%d)", ip, s, prev)
			}
			prev = s
		}
	}
}

func TestUsableIPv4RejectsWhatMustNeverBeShown(t *testing.T) {
	ok := func(s string) net.Addr { return &net.IPNet{IP: net.ParseIP(s)} }
	cases := []struct {
		name string
		addr net.Addr
		want bool
	}{
		{"a real LAN address", ok("192.168.1.5"), true},
		// An interface can carry 127.0.0.0/8 without the loopback FLAG, so
		// skipping loopback interfaces is not enough on its own. Telling somebody
		// their device is 127.0.0.1 is worse than telling them nothing.
		{"loopback", ok("127.0.0.1"), false},
		{"loopback elsewhere in 127/8", ok("127.1.2.3"), false},
		{"the unspecified address", ok("0.0.0.0"), false},
		{"IPv6", ok("fe80::1"), false},
	}
	for _, c := range cases {
		if _, got := usableIPv4(c.addr); got != c.want {
			t.Errorf("%s: usableIPv4 = %v, want %v", c.name, got, c.want)
		}
	}
}
