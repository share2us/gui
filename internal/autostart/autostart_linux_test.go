//go:build linux

package autostart

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLinuxAutostartRoundTrip(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)

	if Enabled() {
		t.Fatal("should not be enabled in a fresh XDG_CONFIG_HOME")
	}
	if err := Enable("/opt/share2us/share2us-gui"); err != nil {
		t.Fatalf("Enable: %v", err)
	}
	if !Enabled() {
		t.Fatal("Enabled() should be true after Enable")
	}
	p := filepath.Join(cfg, "autostart", "share2us-gui-receiver.desktop")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read entry: %v", err)
	}
	if !strings.Contains(string(b), "/opt/share2us/share2us-gui") || !strings.Contains(string(b), "--tray") {
		t.Errorf("entry missing exec/--receive:\n%s", b)
	}

	if err := Disable(); err != nil {
		t.Fatalf("Disable: %v", err)
	}
	if Enabled() {
		t.Fatal("Enabled() should be false after Disable")
	}
}
