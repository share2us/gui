//go:build !windows

package update

// VerifySignature is a no-op off Windows: ApplyUpdate does not execute a
// downloaded binary on other platforms (it opens the release page instead), so
// there is nothing to Authenticode-verify.
func VerifySignature(string) error { return nil }
