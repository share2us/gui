//go:build store

package update

import "errors"

// VerifySignature is unreachable in the Store build (nothing is ever downloaded
// or executed for self-update). It fails closed to guarantee the Store binary
// can never launch an unverified artifact.
func VerifySignature(string) error {
	return errors.New("updates are managed by the Microsoft Store")
}
