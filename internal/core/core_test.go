package core

import (
	"archive/zip"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestDirectoryZipName(t *testing.T) {
	cases := map[string]string{
		"/home/user/photos": "photos.zip",
		"docs":              "docs.zip",
		".":                 "folder.zip",
	}
	for in, want := range cases {
		if got := directoryZipName(in); got != want {
			t.Errorf("directoryZipName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestZipDirectoryRoundTrip(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "a.txt"), "alpha")
	writeFile(t, filepath.Join(root, "sub", "b.txt"), "beta")

	zipped, err := zipDirectory(root)
	if err != nil {
		t.Fatalf("zipDirectory: %v", err)
	}
	defer os.Remove(zipped)

	zr, err := zip.OpenReader(zipped)
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	defer zr.Close()

	var names []string
	for _, f := range zr.File {
		names = append(names, f.Name)
	}
	sort.Strings(names)
	want := []string{"a.txt", "sub/", "sub/b.txt"}
	if len(names) != len(want) {
		t.Fatalf("zip entries = %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("zip entries = %v, want %v", names, want)
		}
	}
}

func TestUniquePath(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "file.txt")
	if got := uniquePath(p); got != p {
		t.Fatalf("unique on free path = %q, want %q", got, p)
	}
	writeFile(t, p, "x")
	if got := uniquePath(p); got != filepath.Join(dir, "file (1).txt") {
		t.Fatalf("unique on taken path = %q, want file (1).txt", got)
	}
}

func TestResolveExpiry(t *testing.T) {
	if _, noExpiry, err := resolveExpiry("30d", true); err != nil || !noExpiry {
		t.Fatalf("keep should set no-expiry, got noExpiry=%v err=%v", noExpiry, err)
	}
	if in, noExpiry, err := resolveExpiry("", false); err != nil || noExpiry || in != "" {
		t.Fatalf("empty should defer to server default, got in=%q noExpiry=%v err=%v", in, noExpiry, err)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
