//go:build windows

package clip

import (
	"errors"
	"strings"
	"sync"

	"golang.design/x/clipboard"
)

var (
	initOnce sync.Once
	initErr  error
)

// ensure initialises the clipboard backend once (CGO-free on Windows).
func ensure() error {
	initOnce.Do(func() { initErr = clipboard.Init() })
	return initErr
}

// Peek reports what shareable content is on the clipboard, preferring an image.
func Peek() (Suggestion, error) {
	if err := ensure(); err != nil {
		return Suggestion{Kind: "none"}, err
	}
	if img := clipboard.Read(clipboard.FmtImage); len(img) > 0 {
		return Suggestion{Kind: "image", Preview: "Image on the clipboard", Ext: "png"}, nil
	}
	if txt := clipboard.Read(clipboard.FmtText); len(txt) > 0 {
		if s := strings.TrimSpace(string(txt)); s != "" {
			return Suggestion{Kind: "text", Preview: snippet(s), Ext: extForText(s)}, nil
		}
	}
	return Suggestion{Kind: "none"}, nil
}

// Read returns the raw bytes + extension for the clipboard content of kind
// ("image" | "text").
func Read(kind string) (data []byte, ext string, err error) {
	if err := ensure(); err != nil {
		return nil, "", err
	}
	switch kind {
	case "image":
		b := clipboard.Read(clipboard.FmtImage)
		if len(b) == 0 {
			return nil, "", errors.New("no image on the clipboard")
		}
		return b, "png", nil
	case "text":
		b := clipboard.Read(clipboard.FmtText)
		if len(b) == 0 {
			return nil, "", errors.New("no text on the clipboard")
		}
		return b, extForText(string(b)), nil
	default:
		return nil, "", errors.New("unknown clipboard kind")
	}
}
