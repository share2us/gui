//go:build linux

// Package shell's Linux integration. There is no single "right-click Share" API
// on Linux — each file manager differs — so we write dep-free, file-based
// integrations that cover the common desktops, plus a universal "Open With"
// entry as a fallback:
//
//   - KDE / Dolphin : a ServiceMenu .desktop  -> "s2u ▸ Share" on files & folders
//   - Nemo (Cinnamon): a .nemo_action          -> "Share with s2u"
//   - Any file manager: an applications/*.desktop -> "Open With → Share2Us"
//
// GNOME / Nautilus has no dep-free right-click hook (its MenuProvider needs
// python3-nautilus); those users get the "Open With" entry until a Nautilus
// extension is added. All files live under XDG_DATA_HOME, so nothing needs admin.
package shell

import (
	"fmt"
	"os"
	"path/filepath"
)

const linuxAppID = "share2us-gui"

// dataHome resolves $XDG_DATA_HOME (default ~/.local/share).
func dataHome() string {
	if d := os.Getenv("XDG_DATA_HOME"); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share")
}

// integrationFiles maps each file's path to its contents for the given exe.
func integrationFiles(exePath string) map[string]string {
	dh := dataHome()
	appEntry := filepath.Join(dh, "applications", linuxAppID+".desktop")
	kdeMenu := filepath.Join(dh, "kio", "servicemenus", linuxAppID+".desktop")
	nemoAction := filepath.Join(dh, "nemo", "actions", linuxAppID+".nemo_action")

	return map[string]string{
		appEntry: "" +
			"[Desktop Entry]\n" +
			"Type=Application\n" +
			"Name=Share2Us\n" +
			"Comment=Share files with Share2Us\n" +
			fmt.Sprintf("Exec=%q share %%F\n", exePath) +
			"Icon=" + linuxAppID + "\n" +
			"Terminal=false\n" +
			"NoDisplay=true\n" +
			"MimeType=application/octet-stream;inode/directory;\n" +
			"Categories=Utility;Network;\n",

		kdeMenu: "" +
			"[Desktop Entry]\n" +
			"Type=Service\n" +
			"MimeType=all/all;\n" +
			"Actions=share2usShare;\n" +
			"X-KDE-Submenu=s2u\n" +
			"X-KDE-Priority=TopLevel\n" +
			"\n" +
			"[Desktop Action share2usShare]\n" +
			"Name=Share\n" +
			"Icon=" + linuxAppID + "\n" +
			fmt.Sprintf("Exec=%q share %%F\n", exePath),

		nemoAction: "" +
			"[Nemo Action]\n" +
			"Name=Share with s2u\n" +
			"Comment=Share via Share2Us\n" +
			fmt.Sprintf("Exec=%q share %%F\n", exePath) +
			"Icon-Name=" + linuxAppID + "\n" +
			"Selection=any\n" +
			"Extensions=any;\n" +
			"Quote=double\n",
	}
}

// Install writes the Linux file-manager integration files for exePath (defaults
// to the running executable). Idempotent.
func Install(exePath string) error {
	if exePath == "" {
		var err error
		exePath, err = os.Executable()
		if err != nil {
			return fmt.Errorf("resolve executable: %w", err)
		}
	}
	for path, content := range integrationFiles(exePath) {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// Uninstall removes the integration files. Missing files are ignored.
func Uninstall() error {
	for path := range integrationFiles("x") {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

// Installed reports whether the KDE ServiceMenu (our primary right-click entry)
// is present.
func Installed() bool {
	kdeMenu := filepath.Join(dataHome(), "kio", "servicemenus", linuxAppID+".desktop")
	_, err := os.Stat(kdeMenu)
	return err == nil
}
