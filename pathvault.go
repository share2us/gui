package main

import (
	"errors"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// pathVault records the file and folder paths the USER actually chose, so the
// bindings can refuse any other.
//
// Every Wails binding is callable from the renderer, and Share, LanSend,
// StartBroadcast and SetIncomingFolder each took an arbitrary absolute path and
// acted on it. No injection sink was found -- external links go through
// BrowserOpenURL and the pages are ours -- so this is defence in depth: if any
// content ever does reach the webview, "upload /home/me/.ssh/id_ed25519" must
// not be one call away (§AJ #26).
//
// A path enters the vault only from the four places the operating system itself
// gives it to us: the Explorer "Share" verb, a native file drop, the native
// file picker and the native folder picker.
type pathVault struct {
	mu      sync.Mutex
	allowed map[string]bool
}

var errPathNotChosen = errors.New("that file was not one you picked; choose it with the file picker or drag it onto the window")

func (v *pathVault) allow(paths ...string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.allowed == nil {
		v.allowed = map[string]bool{}
	}
	for _, p := range paths {
		if key := vaultKey(p); key != "" {
			v.allowed[key] = true
		}
	}
}

// permits reports whether p was chosen by the user, or sits inside a folder
// they chose (a picked directory is shared with its contents).
func (v *pathVault) permits(p string) bool {
	key := vaultKey(p)
	if key == "" {
		return false
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.allowed[key] {
		return true
	}
	for chosen := range v.allowed {
		if strings.HasPrefix(key, chosen+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

// check returns nil when every path is permitted, else the first refusal.
func (v *pathVault) check(paths []string) error {
	for _, p := range paths {
		if !v.permits(p) {
			return errPathNotChosen
		}
	}
	return nil
}

// vaultKey normalises a path for comparison: cleaned, absolute where possible,
// and case-folded on the platforms whose file systems are.
func vaultKey(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	if abs, err := filepath.Abs(p); err == nil {
		p = abs
	}
	p = filepath.Clean(p)
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		p = strings.ToLower(p)
	}
	return p
}
