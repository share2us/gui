//go:build !windows

package clip

import "errors"

// errUnavailable: off Windows the app relies on the browser Ctrl+V paste path
// instead of a backend clipboard read (avoids pulling X11/Cocoa + CGO into the
// Linux/macOS builds).
var errUnavailable = errors.New("clipboard read is only available on Windows")

// Peek reports nothing shareable off Windows.
func Peek() (Suggestion, error) { return Suggestion{Kind: "none"}, errUnavailable }

// Read is unavailable off Windows.
func Read(string) ([]byte, string, error) { return nil, "", errUnavailable }
