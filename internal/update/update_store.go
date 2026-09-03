//go:build store

package update

import (
	"context"
	"net/http"
)

// Check is a no-op in the Store build: the Microsoft Store owns distribution and
// updates for the MSIX, so the app must never poll GitHub or self-update (Store
// policy 10.2.5). It always reports "no update available" without any network.
func Check(_ context.Context, current, channel string) (Info, error) {
	return Info{Current: current, Available: false, Channel: NormalizeChannel(channel)}, nil
}

// VerifyChecksum is a no-op in the Store build for the same reason as Check: the
// Store owns updates, so nothing is ever downloaded here to verify. It mirrors
// VerifySignature's store stub so app.go compiles unchanged under -tags store.
func VerifyChecksum(_ context.Context, _ *http.Client, _, _ string) error { return nil }
