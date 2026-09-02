//go:build !store

package update

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func writeTemp(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "Setup.exe")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// sha256("installer bytes")
const knownSum = "3f2dc7b2a0a1cb5e0a0d0b0a1e4a0a0f0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a"

func TestVerifyChecksumAcceptsMatchAndRejectsMismatch(t *testing.T) {
	path := writeTemp(t, "installer bytes")
	real, err := fileSHA256(path)
	if err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/good":
			_, _ = w.Write([]byte(real + "\n"))
		case "/sha256sum-style":
			// `sha256sum` output is "<hash>  <filename>"; both forms must parse.
			_, _ = w.Write([]byte(real + "  Setup.exe\n"))
		case "/wrong":
			_, _ = w.Write([]byte(knownSum))
		case "/junk":
			_, _ = w.Write([]byte("not a hash"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	client := srv.Client()

	for _, ok := range []string{"/good", "/sha256sum-style"} {
		if err := VerifyChecksum(context.Background(), client, path, srv.URL+ok); err != nil {
			t.Fatalf("%s should verify: %v", ok, err)
		}
	}
	for _, bad := range []string{"/wrong", "/junk", "/missing"} {
		if err := VerifyChecksum(context.Background(), client, path, srv.URL+bad); err == nil {
			t.Fatalf("%s must be refused", bad)
		}
	}
}

func TestVerifyChecksumRefusesWhenNoChecksumPublished(t *testing.T) {
	// A release with no .sha256 must NOT silently skip verification — that is how
	// an integrity gate quietly becomes decorative.
	path := writeTemp(t, "installer bytes")
	if err := VerifyChecksum(context.Background(), http.DefaultClient, path, ""); err == nil {
		t.Fatal("an absent checksum must fail closed, not pass")
	}
}

func TestInfoResolvesChecksumSibling(t *testing.T) {
	rel := ghRelease{
		TagName: "v20260902120000",
		Assets: []ghAsset{
			{Name: "Share2Us-Setup-20260902120000.exe", URL: "https://example.test/Setup.exe"},
			{Name: "Share2Us-Setup-20260902120000.exe.sha256", URL: "https://example.test/Setup.exe.sha256"},
		},
	}
	info := infoFrom(rel, "20260101000000", "windows", "amd64")
	if info.AssetURL != "https://example.test/Setup.exe" {
		t.Fatalf("AssetURL = %q", info.AssetURL)
	}
	if info.SHA256URL != "https://example.test/Setup.exe.sha256" {
		t.Fatalf("SHA256URL = %q — the updater would have nothing to verify against", info.SHA256URL)
	}
}

func TestInfoLeavesChecksumEmptyWhenReleaseHasNone(t *testing.T) {
	// This is the state every existing release is in, which is why the updater
	// must fail closed rather than assume a hash exists.
	rel := ghRelease{
		TagName: "v20260902120000",
		Assets:  []ghAsset{{Name: "Share2Us-Setup-20260902120000.exe", URL: "https://example.test/Setup.exe"}},
	}
	if got := infoFrom(rel, "20260101000000", "windows", "amd64").SHA256URL; got != "" {
		t.Fatalf("SHA256URL = %q, want empty", got)
	}
}
