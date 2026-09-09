package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// §AJ #26: every Wails binding is callable from the renderer, and these took an
// arbitrary absolute path. No injection sink exists today, so this is defence in
// depth: content that ever reaches the webview must not be one call away from
// "upload my ssh key".
func TestBindingsRefusePathsTheUserNeverChose(t *testing.T) {
	dir := t.TempDir()
	picked := filepath.Join(dir, "holiday.jpg")
	secret := filepath.Join(dir, "id_ed25519")

	a := NewApp([]string{picked})

	if err := a.chosen.check([]string{picked}); err != nil {
		t.Fatalf("a path from the Explorer Share verb was refused: %v", err)
	}
	if err := a.chosen.check([]string{secret}); err == nil {
		t.Fatal("an arbitrary path was accepted")
	}
	// A refusal explains what to do rather than just failing.
	err := a.chosen.check([]string{secret})
	if !strings.Contains(err.Error(), "picker") {
		t.Fatalf("unhelpful refusal: %v", err)
	}

	// The picker and a drop are the other ways a path becomes the user's choice.
	a.chosen.allow(secret)
	if err := a.chosen.check([]string{secret}); err != nil {
		t.Fatalf("a picked path was still refused: %v", err)
	}
}

// Picking a FOLDER covers what is inside it: that is what choosing a folder
// means, and the receive folder is picked exactly that way.
func TestChoosingAFolderCoversItsContents(t *testing.T) {
	dir := t.TempDir()
	var v pathVault
	v.allow(dir)

	if !v.permits(filepath.Join(dir, "inside.txt")) {
		t.Fatal("a file inside a chosen folder was refused")
	}
	if !v.permits(filepath.Join(dir, "nested", "deeper.txt")) {
		t.Fatal("a file nested in a chosen folder was refused")
	}
	// A sibling whose name merely starts with the folder's is NOT inside it.
	if v.permits(dir + "-evil/secret.txt") {
		t.Fatal("a sibling directory sharing the prefix was treated as inside")
	}
}

// The same file named differently is the same file.
func TestVaultNormalisesPaths(t *testing.T) {
	dir := t.TempDir()
	var v pathVault
	v.allow(filepath.Join(dir, "a.txt"))
	if !v.permits(filepath.Join(dir, ".", "a.txt")) {
		t.Fatal("a cleanable path was refused")
	}
	if !v.permits(filepath.Join(dir, "sub", "..", "a.txt")) {
		t.Fatal("a path with .. was refused")
	}
	if v.permits("") {
		t.Fatal("the empty path was permitted")
	}
}

// Clearing the receive folder ("ask each time") must stay possible.
func TestSetIncomingFolderStillAcceptsTheEmptyString(t *testing.T) {
	var v pathVault
	if err := v.check([]string{""}); err == nil {
		t.Fatal("the vault should not vouch for the empty path; the caller special-cases it")
	}
}
