package core

import "testing"

// A signed-in browser is not a device. It has no keypair and cannot be given
// one, so it can never receive a sealed send, and the app has no action that
// applies to it. It used to be listed saying "can't receive yet - sign in with
// Share2Us on it", which nobody can act on: there is nothing to install on
// Chrome.
func TestIsBrowserRecognisesAWebSession(t *testing.T) {
	for _, kind := range []string{"web", "WEB", " web "} {
		if !isBrowser(kind) {
			t.Errorf("isBrowser(%q) = false, want true", kind)
		}
	}
	for _, kind := range []string{"cli", "gui", "", "webhook", "android"} {
		if isBrowser(kind) {
			t.Errorf("isBrowser(%q) = true, want false: only a web session is a browser", kind)
		}
	}
}
