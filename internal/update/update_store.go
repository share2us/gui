//go:build store

package update

import "context"

// Check is a no-op in the Store build: the Microsoft Store owns distribution and
// updates for the MSIX, so the app must never poll GitHub or self-update (Store
// policy 10.2.5). It always reports "no update available" without any network.
func Check(_ context.Context, current string) (Info, error) {
	return Info{Current: current, Available: false}, nil
}
