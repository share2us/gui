//go:build windows

// Package shell registers (and removes) the Explorer right-click integration.
//
// It uses a per-user registry "cascading verb" — no COM DLL, no admin. The
// result is: right-click a file or folder -> "s2u" -> "Share", which launches
//
//	share2us-windows.exe share "<selected path>"
//
// This works on Windows 10 (shown inline in the context menu) and Windows 11
// (shown under "Show more options" / the legacy menu). A modern top-level Win11
// entry would need an IExplorerCommand COM extension — deferred by design.
package shell

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows/registry"
)

const (
	// menuKeyName is our stable registry key name under each shell root.
	menuKeyName = "Share2Us"
	// menuLabel is the parent cascading entry the user sees.
	menuLabel = "s2u"
	// verbLabel is the single action under it (more verbs can be added later).
	verbLabel = "Share"
)

// shellRoots are the two shell parents we hook: any file (*) and any directory.
var shellRoots = []string{
	`Software\Classes\*\shell\` + menuKeyName,
	`Software\Classes\Directory\shell\` + menuKeyName,
}

// Install registers the context-menu entry for the current user, pointing at
// exePath (defaults to the running executable). Idempotent.
func Install(exePath string) error {
	if exePath == "" {
		var err error
		exePath, err = os.Executable()
		if err != nil {
			return fmt.Errorf("resolve executable: %w", err)
		}
	}
	command := fmt.Sprintf(`"%s" share "%%1"`, exePath)
	icon := fmt.Sprintf(`"%s",0`, exePath)
	for _, root := range shellRoots {
		if err := writeCascadingVerb(root, icon, command); err != nil {
			return fmt.Errorf("register %s: %w", root, err)
		}
	}
	return nil
}

// writeCascadingVerb builds root ("s2u", with an empty SubCommands to mark it a
// static cascading menu) and root\shell\Share\command (the launch line).
func writeCascadingVerb(root, icon, command string) error {
	access := uint32(registry.CREATE_SUB_KEY | registry.SET_VALUE)

	parent, _, err := registry.CreateKey(registry.CURRENT_USER, root, access)
	if err != nil {
		return err
	}
	defer parent.Close()
	if err := parent.SetStringValue("MUIVerb", menuLabel); err != nil {
		return err
	}
	if err := parent.SetStringValue("Icon", icon); err != nil {
		return err
	}
	// An (empty) SubCommands value tells the shell to build a cascading submenu
	// from the child \shell verbs below.
	if err := parent.SetStringValue("SubCommands", ""); err != nil {
		return err
	}

	verbPath := root + `\shell\` + verbLabel
	verb, _, err := registry.CreateKey(registry.CURRENT_USER, verbPath, access)
	if err != nil {
		return err
	}
	defer verb.Close()
	if err := verb.SetStringValue("MUIVerb", verbLabel); err != nil {
		return err
	}
	if err := verb.SetStringValue("Icon", icon); err != nil {
		return err
	}

	cmd, _, err := registry.CreateKey(registry.CURRENT_USER, verbPath+`\command`, access)
	if err != nil {
		return err
	}
	defer cmd.Close()
	return cmd.SetStringValue("", command)
}

// Uninstall removes the context-menu entry for the current user. Missing keys
// are ignored so it is safe to call unconditionally.
func Uninstall() error {
	for _, root := range shellRoots {
		// Deepest keys first: DeleteKey refuses a key that still has subkeys.
		_ = registry.DeleteKey(registry.CURRENT_USER, root+`\shell\`+verbLabel+`\command`)
		_ = registry.DeleteKey(registry.CURRENT_USER, root+`\shell\`+verbLabel)
		_ = registry.DeleteKey(registry.CURRENT_USER, root+`\shell`)
		_ = registry.DeleteKey(registry.CURRENT_USER, root)
	}
	return nil
}

// Installed reports whether the file-context entry is currently registered.
func Installed() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, shellRoots[0]+`\shell\`+verbLabel+`\command`, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	_ = k.Close()
	return true
}
