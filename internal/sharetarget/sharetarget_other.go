//go:build !windows

package sharetarget

// Activation is a no-op off Windows (the Share sheet is a Windows feature).
func Activation() (paths []string, ok bool) { return nil, false }
