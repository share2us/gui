package main

import (
	"context"
	"sync"
	"testing"
	"time"
)

// Reproduces the report: pause -> resume works, but a SECOND pause/cancel on the
// resumed send "kept going". Drives the real LanSend/PauseSend/ResumeSend/CancelSend
// state machine with a fake sender that blocks until its context is cancelled.
func TestSendPauseResumePause(t *testing.T) {
	a := &App{ctx: context.Background()}
	var mu sync.Mutex
	var ended []string
	a.emit = func(event string, data map[string]any) {
		if event == "lan-send-ended" {
			mu.Lock()
			ended = append(ended, data["status"].(string))
			mu.Unlock()
		}
	}
	started := make(chan struct{}, 8)
	orig := sendOneFn
	sendOneFn = func(ctx context.Context, _, _, _ string, onProg func(int64, int64)) error {
		onProg(10, 100) // some bytes moved, so a cancel classifies as paused/cancelled not failed
		started <- struct{}{}
		<-ctx.Done()
		return ctx.Err()
	}
	defer func() { sendOneFn = orig }()

	waitEnded := func(n int) {
		for i := 0; i < 200; i++ {
			mu.Lock()
			got := len(ended)
			mu.Unlock()
			if got >= n {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatalf("timed out waiting for %d lan-send-ended events; got %v", n, ended)
	}

	a.chosen.allow("A")
	go a.LanSend([]string{"A"}, "127.0.0.1:1", "")
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("first send never started")
	}

	// First pause -> resume (the part the user said works).
	a.PauseSend("send-1")
	waitEnded(1)
	a.ResumeSend("send-1")
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("resumed send never started")
	}

	// Second pause: this is what "kept going".
	a.PauseSend("send-1")
	waitEnded(2)

	mu.Lock()
	defer mu.Unlock()
	for i, s := range ended {
		if s != "paused" {
			t.Fatalf("ended[%d] = %q, want paused", i, s)
		}
	}
	if len(ended) != 2 {
		t.Fatalf("want 2 paused events, got %v", ended)
	}
}

// A second CANCEL on a resumed send must also stop it and remove the registry entry.
func TestSendResumeThenCancel(t *testing.T) {
	a := &App{ctx: context.Background(), emit: func(string, map[string]any) {}}
	started := make(chan struct{}, 8)
	orig := sendOneFn
	sendOneFn = func(ctx context.Context, _, _, _ string, onProg func(int64, int64)) error {
		onProg(10, 100)
		started <- struct{}{}
		<-ctx.Done()
		return ctx.Err()
	}
	defer func() { sendOneFn = orig }()

	a.chosen.allow("A")
	go a.LanSend([]string{"A"}, "127.0.0.1:1", "")
	<-started
	a.PauseSend("send-1")
	a.ResumeSend("send-1")
	<-started
	a.CancelSend("send-1")
	// Give the resumed goroutine a moment to unwind.
	time.Sleep(100 * time.Millisecond)
	a.xferMu.Lock()
	_, stillThere := a.sendXfers["send-1"]
	a.xferMu.Unlock()
	if stillThere {
		t.Fatal("a cancelled send should be removed from the registry")
	}
}
