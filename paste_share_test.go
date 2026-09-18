package main

import (
	"encoding/base64"
	"strings"
	"testing"
)

// Pasting text or a link produced a file the app then refused to share. The path
// vault (§AJ #26) makes Share reject any path the user did not pick, and the two
// clipboard doors -- AddPasted for Ctrl+V and AddClipboard for the "Share copied
// text" chip -- wrote a temp file without permitting it. So the file was created,
// the UI listed it, and the share silently would not go, which read to the user
// as "it still asks for a file".
//
// Every existing test covers a path source that already vouches for itself, which
// is exactly how this got through: there was no test that took a path the app had
// MADE and then tried to share it.

func TestPastedContentCanActuallyBeShared(t *testing.T) {
	a := &App{}
	text := "https://example.test/a-link-someone-pasted"

	path, err := a.AddPasted("txt", base64.StdEncoding.EncodeToString([]byte(text)))
	if err != nil {
		t.Fatalf("AddPasted() error = %v", err)
	}
	if path == "" {
		t.Fatal("AddPasted() returned no path")
	}

	// The assertion that matters: the app must accept the file it just created.
	// Returning a path it will not accept is worse than refusing outright,
	// because the refusal arrives later and without a reason.
	if err := a.chosen.check([]string{path}); err != nil {
		t.Fatalf("the app refused a file it created itself: %v", err)
	}
}

// The chip is a separate entry point and was broken the same way. Testing only
// Ctrl+V would have left it broken while looking fixed.
func TestClipboardChipPathIsAlsoPermitted(t *testing.T) {
	a := &App{}
	path, err := a.writeTempShare([]byte("clipboard text"), "txt")
	if err != nil {
		t.Fatalf("writeTempShare() error = %v", err)
	}
	if err := a.chosen.check([]string{path}); err != nil {
		t.Fatalf("clipboard path not permitted: %v", err)
	}
}

// The vault must still refuse a path the app did NOT create. Without this, the
// fix above could have been "permit everything", which would undo §AJ #26.
func TestAnArbitraryPathIsStillRefused(t *testing.T) {
	a := &App{}
	if _, err := a.AddPasted("txt", base64.StdEncoding.EncodeToString([]byte("x"))); err != nil {
		t.Fatalf("AddPasted() error = %v", err)
	}
	err := a.chosen.check([]string{"/etc/passwd"})
	if err == nil {
		t.Fatal("a path the user never chose was permitted -- the vault is not doing its job")
	}
	if !strings.Contains(err.Error(), "not one you picked") {
		t.Errorf("refusal message = %q, want it to explain the path was not chosen", err)
	}
}
