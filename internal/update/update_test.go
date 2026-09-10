// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Hassan Khurram

//go:build !store

package update

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func testRelease() ghRelease {
	return ghRelease{
		TagName: "v20260723120000",
		HTMLURL: "https://github.com/share2us/gui/releases/tag/v20260723120000",
		Assets: []ghAsset{
			{Name: "Share2Us-Setup-20260723120000.exe", URL: "https://x/setup.exe"},
			{Name: "share2us-gui_linux_amd64.tar.gz", URL: "https://x/linux-amd64.tgz"},
			{Name: "share2us-gui_linux_arm64.tar.gz", URL: "https://x/linux-arm64.tgz"},
			{Name: "share2us-gui_darwin_universal.zip", URL: "https://x/mac.zip"},
		},
	}
}

func TestInfoFromAvailability(t *testing.T) {
	rel := testRelease()
	cases := []struct {
		current string
		want    bool
	}{
		{"20260722000000", true},  // older -> update
		{"20260723120000", false}, // same
		{"20260724000000", false}, // newer local build
		{"dev", false},            // dev build never nags
		{"", false},               // unstamped
	}
	for _, c := range cases {
		if got := infoFrom(rel, c.current, "linux", "amd64").Available; got != c.want {
			t.Errorf("current=%q available=%v want %v", c.current, got, c.want)
		}
	}
}

func TestPickAssetPerOS(t *testing.T) {
	rel := testRelease()
	cases := []struct{ goos, goarch, want string }{
		{"windows", "amd64", "Share2Us-Setup-20260723120000.exe"},
		{"linux", "amd64", "share2us-gui_linux_amd64.tar.gz"},
		{"linux", "arm64", "share2us-gui_linux_arm64.tar.gz"},
		{"darwin", "arm64", "share2us-gui_darwin_universal.zip"},
	}
	for _, c := range cases {
		if name, _ := pickAsset(rel, c.goos, c.goarch); name != c.want {
			t.Errorf("%s/%s asset = %q, want %q", c.goos, c.goarch, name, c.want)
		}
	}
}

func TestNewestReleasePrefersHighestNonDraftIncludingPrereleases(t *testing.T) {
	releases := []ghRelease{
		{TagName: "v20260901000000"},                                // stable
		{TagName: "v20260903000000", Prerelease: true},              // newer beta
		{TagName: "v20260904000000", Prerelease: true, Draft: true}, // draft: ignored
		{TagName: "v1", Prerelease: true},                           // malformed: ignored
	}
	rel, ok := newestRelease(releases)
	if !ok || rel.TagName != "v20260903000000" || !rel.Prerelease {
		t.Fatalf("newest = %+v ok=%v", rel, ok)
	}
	// A stable newer than the last beta wins on the beta channel too.
	rel, _ = newestRelease([]ghRelease{{TagName: "v20260905000000"}, {TagName: "v20260903000000", Prerelease: true}})
	if rel.TagName != "v20260905000000" || rel.Prerelease {
		t.Fatalf("newest stable should win: %+v", rel)
	}
	if _, ok := newestRelease([]ghRelease{{TagName: "v2", Draft: true}}); ok {
		t.Fatal("no eligible release should report ok=false")
	}
}

func TestCheckBetaAtAgainstAReleaseList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `[
		  {"tag_name":"v20260901000000","html_url":"https://x/stable","assets":[{"name":"share2us-gui_linux_amd64.tar.gz","browser_download_url":"https://x/stable.tgz"}]},
		  {"tag_name":"v20260903000000","prerelease":true,"html_url":"https://x/beta","assets":[{"name":"share2us-gui_linux_amd64.tar.gz","browser_download_url":"https://x/beta.tgz"},{"name":"share2us-gui_linux_amd64.tar.gz.sha256","browser_download_url":"https://x/beta.tgz.sha256"}]}
		]`)
	}))
	defer srv.Close()
	info, err := checkBetaAt(context.Background(), srv.Client(), srv.URL, "20260902000000", "linux", "amd64")
	if err != nil {
		t.Fatal(err)
	}
	if !info.Available || info.Latest != "20260903000000" || info.Channel != ChannelBeta || !info.Prerelease {
		t.Fatalf("info = %+v", info)
	}
	if info.AssetURL != "https://x/beta.tgz" || info.SHA256URL != "https://x/beta.tgz.sha256" {
		t.Fatalf("assets = %s %s", info.AssetURL, info.SHA256URL)
	}
}

func TestNormalizeChannel(t *testing.T) {
	for in, want := range map[string]string{"": ChannelStable, "stable": ChannelStable, "Beta": ChannelBeta, "nightly": ChannelStable} {
		if got := NormalizeChannel(in); got != want {
			t.Errorf("NormalizeChannel(%q) = %q, want %q", in, got, want)
		}
	}
}

// A release list is bigger than it looks: GitHub inlines every asset of every
// release. The beta channel was dead for weeks because the read cap sat below
// one real page and the JSON came back truncated, which surfaces as a decode
// error and an update check that quietly finds nothing. This pins the cap above
// a page far larger than the one we ask for.
func TestCheckBetaAtReadsAPageLargerThanAMegabyte(t *testing.T) {
	var b strings.Builder
	b.WriteString(`[{"tag_name":"v20260903000000","prerelease":true,"html_url":"https://x/beta","assets":[`)
	b.WriteString(`{"name":"share2us-gui_linux_amd64.tar.gz","browser_download_url":"https://x/beta.tgz"}`)
	// Padding assets, to the shape and bulk GitHub actually returns.
	for i := 0; b.Len() < 3<<20; i++ {
		fmt.Fprintf(&b, `,{"name":"pad-%d-%s.bin","browser_download_url":"https://x/pad-%d-%s.bin"}`,
			i, strings.Repeat("x", 300), i, strings.Repeat("y", 300))
	}
	b.WriteString(`]}]`)
	body := b.String()
	if len(body) <= 1<<20 {
		t.Fatalf("test body is %d bytes, not larger than the old cap", len(body))
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, body)
	}))
	defer srv.Close()

	info, err := checkBetaAt(context.Background(), srv.Client(), srv.URL, "20260902000000", "linux", "amd64")
	if err != nil {
		t.Fatalf("beta check on a %d byte page: %v", len(body), err)
	}
	if !info.Available || info.Latest != "20260903000000" {
		t.Fatalf("info = %+v", info)
	}
}

// The page we ask GitHub for has to fit under the cap with room to spare.
func TestBetaListURLAsksForAShortPage(t *testing.T) {
	if !strings.Contains(defaultReleaseListURL, "per_page=5") {
		t.Fatalf("beta list URL = %q, want a short page", defaultReleaseListURL)
	}
}
