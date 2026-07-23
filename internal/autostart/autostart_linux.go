//go:build linux

// Package autostart, Linux: an XDG autostart .desktop entry (works on GNOME, KDE,
// XFCE, ...), under XDG_CONFIG_HOME/autostart. No admin.
package autostart

import (
	"fmt"
	"os"
	"path/filepath"
)

const desktopName = "share2us-gui-receiver.desktop"

func configHome() string {
	if d := os.Getenv("XDG_CONFIG_HOME"); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config")
}

func entryPath() string {
	return filepath.Join(configHome(), "autostart", desktopName)
}

// Enable writes the autostart entry that runs `<exe> --receive` at login.
func Enable(exePath string) error {
	if exePath == "" {
		var err error
		exePath, err = os.Executable()
		if err != nil {
			return err
		}
	}
	p := entryPath()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	content := "[Desktop Entry]\n" +
		"Type=Application\n" +
		"Name=Share2Us Receiver\n" +
		"Comment=Receive files sent to this device\n" +
		fmt.Sprintf("Exec=%q --tray\n", exePath) +
		"Icon=share2us-gui\n" +
		"Terminal=false\n" +
		"NoDisplay=true\n" +
		"X-GNOME-Autostart-enabled=true\n"
	return os.WriteFile(p, []byte(content), 0o644)
}

// Disable removes the autostart entry. Missing file is not an error.
func Disable() error {
	if err := os.Remove(entryPath()); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// Enabled reports whether the autostart entry exists.
func Enabled() bool {
	_, err := os.Stat(entryPath())
	return err == nil
}
