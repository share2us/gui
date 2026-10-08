package core

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	clicore "github.com/share2us/cli-core"
)

func TestLoginPreservesApprovedSigningIdentity(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	t.Setenv("APPDATA", dir)
	var presented string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/auth/device-codes":
			var req clicore.DeviceCodeRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Error(err)
				w.WriteHeader(400)
				return
			}
			presented = req.SigningPublicKey
			json.NewEncoder(w).Encode(map[string]any{"device_code": "test-login", "interval": 1})
		case "/v1/auth/device-codes/test-login/token":
			json.NewEncoder(w).Encode(map[string]string{"credential": "s2s_test", "device_session_id": "same-device"})
		case "/v1/auth/devices/key":
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		case "/v1/auth/me":
			json.NewEncoder(w).Encode(map[string]string{"email": "login@example.test", "account_id": "test-account"})
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	t.Setenv("SHARE2US_API_BASE", server.URL)
	s, err := StartLogin(t.Context(), "test-device")
	if err != nil {
		t.Fatal(err)
	}
	if presented == "" {
		t.Fatal("login omitted signing identity")
	}
	client, err := s.Wait(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	cred, err := clicore.LoadCredential()
	if err != nil {
		t.Fatal(err)
	}
	if cred.DeviceSigningPublicKey != presented || cred.DeviceSigningPrivateKey != s.signingKey.PrivateKey || client.cred.DeviceSigningPublicKey != presented {
		t.Fatal("login dropped the approved signing identity")
	}
	oldPublic, encryptionPrivate := cred.DeviceSigningPublicKey, cred.DevicePrivateKey
	cred.DeviceSigningPrivateKey = ""
	if err := clicore.SaveCredential(cred); err != nil {
		t.Fatal(err)
	}
	s, err = StartLogin(t.Context(), "test-device")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Wait(t.Context()); err != nil {
		t.Fatal(err)
	}
	cred, err = clicore.LoadCredential()
	if err != nil {
		t.Fatal(err)
	}
	if cred.DeviceSigningPublicKey == oldPublic || cred.DeviceSigningPublicKey != presented || cred.DeviceSigningPrivateKey != s.signingKey.PrivateKey || cred.DevicePrivateKey != encryptionPrivate {
		t.Fatal("GUI re-login did not recover the signing key while preserving encryption")
	}
}
