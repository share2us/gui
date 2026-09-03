//go:build !windows || store

package update

// SetPinnedThumbprint is a no-op off Windows and in Store builds. Authenticode is
// Windows-only, and a Store build has no updater to verify anything for.
func SetPinnedThumbprint(string) {}
