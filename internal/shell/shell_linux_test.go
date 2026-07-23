//go:build linux

package shell

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLinuxInstallUninstall(t *testing.T) {
	data := t.TempDir()
	t.Setenv("XDG_DATA_HOME", data)

	if Installed() {
		t.Fatal("should not be installed in a fresh XDG_DATA_HOME")
	}
	if err := Install("/opt/share2us/share2us-gui"); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if !Installed() {
		t.Fatal("Installed() should be true after Install")
	}

	want := []string{
		filepath.Join(data, "applications", "share2us-gui.desktop"),
		filepath.Join(data, "kio", "servicemenus", "share2us-gui.desktop"),
		filepath.Join(data, "nemo", "actions", "share2us-gui.nemo_action"),
	}
	for _, p := range want {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("expected %s: %v", p, err)
		}
		if !strings.Contains(string(b), "/opt/share2us/share2us-gui") {
			t.Errorf("%s missing exe path", p)
		}
		if !strings.Contains(string(b), "share %F") {
			t.Errorf("%s missing 'share %%F' invocation", p)
		}
	}

	if err := Uninstall(); err != nil {
		t.Fatalf("Uninstall: %v", err)
	}
	if Installed() {
		t.Fatal("Installed() should be false after Uninstall")
	}
	for _, p := range want {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("%s should be removed", p)
		}
	}
}
