// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Hassan Khurram

package incoming_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/share2us/cli-core/lanshare"
	"github.com/share2us/gui/internal/incoming"
)

// A real transfer, over the real protocol, landing in staging.
//
// The unit tests cover the staging bookkeeping with files placed by hand. This
// one sends a file the way another machine does — TLS, the receiver's own
// approval callback, the actual lanshare receive loop — and then asserts the
// arrival is staged and can be filed where the user chooses. It is the step
// between "the pieces work" and "a file someone sent me ends up where I said".
func TestArrivalFromARealTransferIsStagedThenFiled(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	stage, err := incoming.StagePath()
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	listening := make(chan int, 1)
	done := make(chan lanshare.ReceiveResult, 1)
	go func() {
		res, rerr := lanshare.Receive(ctx, lanshare.ReceiveOptions{
			Bind:       "127.0.0.1",
			NoPassword: true,
			DestDir:    stage,
			OnListen:   func(li lanshare.ListenInfo) { listening <- li.Port },
			OnRequest:  func(lanshare.RequestInfo) bool { return true },
		})
		if rerr == nil {
			done <- res
		}
	}()

	var port int
	select {
	case port = <-listening:
	case <-time.After(10 * time.Second):
		t.Fatal("receiver never came up")
	}

	payload := make([]byte, 64*1024)
	if _, err := rand.Read(payload); err != nil {
		t.Fatal(err)
	}
	if _, err := lanshare.Send(ctx, "quarterly-report.pdf", int64(len(payload)), false,
		bytes.NewReader(payload), lanshare.SendOptions{
			Dest:       "127.0.0.1:" + itoa(port),
			SenderName: "hassan-laptop",
		}); err != nil {
		t.Fatalf("send: %v", err)
	}

	var res lanshare.ReceiveResult
	select {
	case res = <-done:
	case <-time.After(15 * time.Second):
		t.Fatal("transfer never completed")
	}

	// This is what App.fileArrival does when no folder is remembered: the file
	// waits, rather than being written somewhere the user never chose.
	if _, err := incoming.Add(res.Name, "hassan-laptop", res.Path, res.Bytes); err != nil {
		t.Fatal(err)
	}

	waiting := incoming.List()
	if len(waiting) != 1 {
		t.Fatalf("expected one arrival waiting, got %d", len(waiting))
	}
	if waiting[0].Name != "quarterly-report.pdf" || waiting[0].From != "hassan-laptop" {
		t.Fatalf("arrival lost its identity: %+v", waiting[0])
	}
	if waiting[0].Size != int64(len(payload)) {
		t.Errorf("size = %d, want %d", waiting[0].Size, len(payload))
	}

	// And filing it puts the real bytes where the user asked.
	dest := filepath.Join(t.TempDir(), "saved.pdf")
	if err := incoming.Claim(waiting[0].ID, dest); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatal("the filed copy does not match what was sent")
	}
	if len(incoming.List()) != 0 {
		t.Error("a filed arrival should stop waiting")
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	return string(b)
}
