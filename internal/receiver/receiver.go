// Package receiver runs the background inbox loop: poll for incoming device/
// contact sends, decrypt them, and save them into the Downloads folder. It is
// platform-independent and takes a Poller (satisfied by core.Client), so it
// unit-tests without a network or a real login.
package receiver

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/share2us/gui/internal/core"
)

// DefaultInterval matches the CLI's `receive --watch` cadence.
const DefaultInterval = 5 * time.Second

// Poller performs one receive cycle into destDir. *core.Client satisfies it.
type Poller interface {
	ReceiveOnce(ctx context.Context, destDir string) ([]core.Received, error)
}

// Event is one cycle's outcome: either files were saved, or an error occurred.
type Event struct {
	Received []core.Received
	Err      error
}

// DownloadsDir resolves the user's Downloads folder: $XDG_DOWNLOAD_DIR when set
// (Linux), otherwise ~/Downloads (covers %USERPROFILE%\Downloads on Windows).
func DownloadsDir() string {
	if d := os.Getenv("XDG_DOWNLOAD_DIR"); d != "" {
		return d
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return filepath.Join(home, "Downloads")
}

// Loop polls immediately and then every interval until ctx is cancelled. onEvent
// fires only for cycles that saved files or errored (quiet cycles are silent), so
// a caller can turn each event straight into a toast/log.
func Loop(ctx context.Context, p Poller, destDir string, interval time.Duration, onEvent func(Event)) {
	if interval <= 0 {
		interval = DefaultInterval
	}
	poll := func() {
		got, err := p.ReceiveOnce(ctx, destDir)
		switch {
		case err != nil:
			if ctx.Err() == nil {
				onEvent(Event{Err: err})
			}
		case len(got) > 0:
			onEvent(Event{Received: got})
		}
	}
	poll()
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			poll()
		}
	}
}
