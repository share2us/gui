// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Hassan Khurram

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
	"github.com/share2us/gui/internal/incoming"
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

// StagingLoop is Loop, except that each arrival is disposed of by
// incoming.Deliver rather than written straight into destDir (§AG D1).
//
// The distinction matters: cloud device-sends used to be written into the
// Downloads folder and announced with a toast, which is precisely the behaviour
// the 2026-09-07 staging decision replaced for LAN transfers — "the user never
// chose, and afterwards had to go looking for it". One arrival path, one rule.
//
// Files are fetched into the staging directory first, so a download that is
// interrupted leaves nothing in a folder the user watches.
func StagingLoop(ctx context.Context, p Poller, interval time.Duration, onEvent func(Event)) {
	staging, err := incoming.StagePath()
	if err != nil {
		// Without a staging directory there is nowhere safe to put anything.
		// Report it rather than silently falling back to writing into Downloads,
		// which is the behaviour being replaced.
		onEvent(Event{Err: err})
		return
	}
	Loop(ctx, p, staging, interval, func(e Event) {
		if e.Err != nil {
			onEvent(e)
			return
		}
		delivered := make([]core.Received, 0, len(e.Received))
		for _, r := range e.Received {
			got, derr := incoming.Deliver(r.FileName, r.From, r.SavedTo, 0)
			if derr != nil && got.Item.ID == "" {
				onEvent(Event{Err: derr})
				continue
			}
			r.SavedTo = got.Path
			delivered = append(delivered, r)
		}
		if len(delivered) > 0 {
			onEvent(Event{Received: delivered})
		}
	})
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
