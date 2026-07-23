package receiver

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/share2us/gui/internal/core"
)

// fakePoller returns a scripted sequence of results, one per ReceiveOnce call,
// then empty. It cancels the context once the script is exhausted so Loop stops.
type fakePoller struct {
	mu     sync.Mutex
	calls  int
	script [][]core.Received
	errs   []error
	cancel context.CancelFunc
}

func (f *fakePoller) ReceiveOnce(_ context.Context, _ string) ([]core.Received, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	i := f.calls
	f.calls++
	if i >= len(f.script) {
		if f.cancel != nil {
			f.cancel()
		}
		return nil, nil
	}
	var err error
	if i < len(f.errs) {
		err = f.errs[i]
	}
	return f.script[i], err
}

func TestLoopDeliversAndStops(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	f := &fakePoller{
		script: [][]core.Received{
			{{PublicID: "a", SavedTo: "/dl/a.txt", From: "phone"}},
			nil,
			{{PublicID: "b", SavedTo: "/dl/b.txt"}},
		},
		errs:   []error{nil, errors.New("boom"), nil},
		cancel: cancel,
	}

	var (
		mu    sync.Mutex
		saved []core.Received
		errN  int
	)
	done := make(chan struct{})
	go func() {
		Loop(ctx, f, "/dl", time.Millisecond, func(e Event) {
			mu.Lock()
			defer mu.Unlock()
			if e.Err != nil {
				errN++
				return
			}
			saved = append(saved, e.Received...)
		})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Loop did not stop after context cancel")
	}

	mu.Lock()
	defer mu.Unlock()
	if len(saved) != 2 {
		t.Fatalf("expected 2 saved files, got %d", len(saved))
	}
	if errN != 1 {
		t.Fatalf("expected 1 error event, got %d", errN)
	}
}

func TestDownloadsDirXDGOverride(t *testing.T) {
	t.Setenv("XDG_DOWNLOAD_DIR", "/custom/dl")
	if got := DownloadsDir(); got != "/custom/dl" {
		t.Fatalf("DownloadsDir() = %q, want /custom/dl", got)
	}
}
