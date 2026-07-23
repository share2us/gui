// Package lanid manages this device's persistent LAN identity (an Ed25519
// keypair) and the receiver-side trusted-devices store. Trust is keyed by the
// sender's verified public-key fingerprint (never its name or IP), so a device
// recognised once needs no per-send verify code — and can be revoked.
package lanid

import (
	"crypto/ed25519"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/share2us/cli-core/lanshare"
)

// configDir returns %AppData%/share2us (the same base cli-core uses for
// credentials), creating it 0700 if needed.
func configDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "share2us")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

// ---- device identity --------------------------------------------------------

type identityFile struct {
	Priv []byte `json:"priv"` // ed25519 private key (64 bytes)
}

var (
	idOnce sync.Once
	idKey  ed25519.PrivateKey
	idErr  error
)

// Identity returns this device's persistent Ed25519 identity, creating and
// saving it (0600) on first use.
func Identity() (ed25519.PrivateKey, error) {
	idOnce.Do(func() { idKey, idErr = loadOrCreateIdentity() })
	return idKey, idErr
}

func loadOrCreateIdentity() (ed25519.PrivateKey, error) {
	dir, err := configDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "lan_identity.json")
	if data, err := os.ReadFile(path); err == nil {
		var f identityFile
		if json.Unmarshal(data, &f) == nil && len(f.Priv) == ed25519.PrivateKeySize {
			return ed25519.PrivateKey(f.Priv), nil
		}
	}
	_, priv, err := ed25519.GenerateKey(nil) // nil => crypto/rand
	if err != nil {
		return nil, err
	}
	if data, merr := json.Marshal(identityFile{Priv: priv}); merr == nil {
		_ = os.WriteFile(path, data, 0o600)
	}
	return priv, nil
}

// Fingerprint / Code identify this device to peers.
func Fingerprint() string {
	k, err := Identity()
	if err != nil {
		return ""
	}
	return lanshare.IdentityFingerprint(k.Public().(ed25519.PublicKey))
}

func Code() string { return lanshare.VerifyCode(Fingerprint()) }

// ---- trusted-devices store --------------------------------------------------

// TrustedDevice is a receiver-side trusted sender. Mode is "ask" (still confirm
// each transfer, no code) or "auto" (accept silently).
type TrustedDevice struct {
	Fingerprint string `json:"fingerprint"`
	Name        string `json:"name"`
	Mode        string `json:"mode"`
}

type trustStore struct {
	mu   sync.Mutex
	path string
	m    map[string]TrustedDevice
}

var (
	tsOnce sync.Once
	ts     *trustStore
	tsErr  error
)

func store() (*trustStore, error) {
	tsOnce.Do(func() {
		dir, err := configDir()
		if err != nil {
			tsErr = err
			return
		}
		s := &trustStore{path: filepath.Join(dir, "lan_trusted.json"), m: map[string]TrustedDevice{}}
		if data, rerr := os.ReadFile(s.path); rerr == nil {
			var list []TrustedDevice
			if json.Unmarshal(data, &list) == nil {
				for _, d := range list {
					if d.Fingerprint != "" {
						s.m[d.Fingerprint] = d
					}
				}
			}
		}
		ts = s
	})
	return ts, tsErr
}

// saveLocked writes the store atomically (caller holds s.mu).
func (s *trustStore) saveLocked() {
	list := make([]TrustedDevice, 0, len(s.m))
	for _, d := range s.m {
		list = append(list, d)
	}
	if data, err := json.MarshalIndent(list, "", "  "); err == nil {
		tmp := s.path + ".tmp"
		if os.WriteFile(tmp, data, 0o600) == nil {
			_ = os.Rename(tmp, s.path)
		}
	}
}

// Lookup returns the trusted device for a fingerprint (ok=false if untrusted or
// the fingerprint is empty/anonymous).
func Lookup(fingerprint string) (TrustedDevice, bool) {
	if fingerprint == "" {
		return TrustedDevice{}, false
	}
	s, err := store()
	if err != nil {
		return TrustedDevice{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.m[fingerprint]
	return d, ok
}

// Trust adds/updates a trusted device. Any mode other than "auto" is stored as
// "ask" (never silently auto-accept unless explicitly chosen).
func Trust(fingerprint, name, mode string) error {
	if fingerprint == "" {
		return nil
	}
	s, err := store()
	if err != nil {
		return err
	}
	if mode != "auto" {
		mode = "ask"
	}
	s.mu.Lock()
	s.m[fingerprint] = TrustedDevice{Fingerprint: fingerprint, Name: name, Mode: mode}
	s.saveLocked()
	s.mu.Unlock()
	return nil
}

// Untrust removes a device from the trust store (revoke).
func Untrust(fingerprint string) error {
	s, err := store()
	if err != nil {
		return err
	}
	s.mu.Lock()
	delete(s.m, fingerprint)
	s.saveLocked()
	s.mu.Unlock()
	return nil
}

// List returns the trusted devices, sorted by name.
func List() []TrustedDevice {
	s, err := store()
	if err != nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	list := make([]TrustedDevice, 0, len(s.m))
	for _, d := range s.m {
		list = append(list, d)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })
	return list
}
