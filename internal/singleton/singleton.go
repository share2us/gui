// Package singleton is a small cross-platform single-instance guard built on a
// loopback listener: the first process to bind the port owns the instance, and
// the OS frees the port automatically when that process exits (so there are no
// stale locks to clean up after a crash). We use it to keep exactly one
// background tray process alive no matter how many times the app is launched.
package singleton

import "net"

// Lock holds the single-instance claim. Keep it alive for the whole process;
// the claim is released when the process exits (or Release is called).
type Lock struct{ ln net.Listener }

// Acquire tries to become the single instance on the given loopback port.
// It returns (lock, true) for the first/only instance and (nil, false) when
// another instance already holds it. Binding is restricted to 127.0.0.1 so it
// never trips a Windows Firewall prompt and is never reachable off-box.
func Acquire(port string) (*Lock, bool) {
	ln, err := net.Listen("tcp", "127.0.0.1:"+port)
	if err != nil {
		return nil, false
	}
	return &Lock{ln: ln}, true
}

// Release frees the claim early. This also happens automatically when the
// process exits, so callers can simply hold the Lock and never call it.
func (l *Lock) Release() {
	if l != nil && l.ln != nil {
		_ = l.ln.Close()
	}
}
