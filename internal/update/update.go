//go:build !store

// Package update checks GitHub Releases for a newer Share2Us build. Versions are
// UTC-timestamp strings (main.buildVersion), so "newer" is a plain string compare
// of the release tag against the running build.
//
// This file (and the network/installer machinery) is compiled out of the Store
// build (`-tags store`), which ships update_store.go instead — the Microsoft
// Store distributes and updates the MSIX itself (Store policy 10.2.5).
package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"
	"time"
)

const (
	// stable: GitHub's "latest" release, which is never a pre-release.
	defaultReleasesURL = "https://api.github.com/repos/share2us/gui/releases/latest"
	// beta: the full list (pre-releases included), newest picked by tag.
	defaultReleaseListURL = "https://api.github.com/repos/share2us/gui/releases?per_page=30"
)

type ghAsset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

type ghRelease struct {
	TagName    string    `json:"tag_name"`
	HTMLURL    string    `json:"html_url"`
	Draft      bool      `json:"draft"`
	Prerelease bool      `json:"prerelease"`
	Assets     []ghAsset `json:"assets"`
}

// Check queries GitHub for the newest release on the channel for the running
// OS/arch. channel is "stable" (or "") or "beta"; see NormalizeChannel.
func Check(ctx context.Context, current, channel string) (Info, error) {
	client := &http.Client{Timeout: 12 * time.Second}
	if NormalizeChannel(channel) == ChannelBeta {
		return checkBetaAt(ctx, client, defaultReleaseListURL, current, runtime.GOOS, runtime.GOARCH)
	}
	return checkAt(ctx, client, defaultReleasesURL, current, runtime.GOOS, runtime.GOARCH)
}

func checkAt(ctx context.Context, client *http.Client, url, current, goos, goarch string) (Info, error) {
	var rel ghRelease
	if err := getJSON(ctx, client, url, &rel); err != nil {
		return Info{}, err
	}
	info := infoFrom(rel, current, goos, goarch)
	info.Channel = ChannelStable
	return info, nil
}

// checkBetaAt lists releases and offers the newest non-draft one, pre-release
// or not. A stable newer than the last beta wins, so a beta install is never
// stranded on an old build.
func checkBetaAt(ctx context.Context, client *http.Client, url, current, goos, goarch string) (Info, error) {
	var releases []ghRelease
	if err := getJSON(ctx, client, url, &releases); err != nil {
		return Info{}, err
	}
	rel, ok := newestRelease(releases)
	if !ok {
		return Info{}, fmt.Errorf("update check: no published release")
	}
	info := infoFrom(rel, current, goos, goarch)
	info.Channel = ChannelBeta
	info.Prerelease = rel.Prerelease
	return info, nil
}

// newestRelease picks the highest build version among non-draft releases. Tags
// are "v<UTC timestamp>", fixed width, so after a length check plain string order
// is chronological; a malformed tag never wins.
func newestRelease(releases []ghRelease) (ghRelease, bool) {
	var best ghRelease
	found := false
	for _, r := range releases {
		v := buildVersion(r.TagName)
		if r.Draft || v == "" {
			continue
		}
		if !found || versionLess(buildVersion(best.TagName), v) {
			best, found = r, true
		}
	}
	return best, found
}

func buildVersion(tag string) string {
	v := strings.TrimPrefix(strings.TrimSpace(tag), "v")
	if v == "" {
		return ""
	}
	for _, r := range v {
		if r < '0' || r > '9' {
			return ""
		}
	}
	return v
}

func versionLess(a, b string) bool {
	if len(a) != len(b) {
		return len(a) < len(b)
	}
	return a < b
}

func getJSON(ctx context.Context, client *http.Client, url string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "share2us-gui")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("update check: HTTP %d", resp.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(out)
}

func infoFrom(rel ghRelease, current, goos, goarch string) Info {
	latest := strings.TrimPrefix(strings.TrimSpace(rel.TagName), "v")
	info := Info{Current: current, Latest: latest, Page: rel.HTMLURL}
	// A dev/unstamped build never nags; otherwise newer = lexicographically greater
	// (timestamps are fixed-width, so this equals numeric order).
	info.Available = current != "" && current != "dev" && latest != "" && latest > current
	info.AssetName, info.AssetURL = pickAsset(rel, goos, goarch)
	if info.AssetName != "" {
		info.SHA256URL = assetURL(rel, info.AssetName+".sha256")
	}
	return info
}

// assetURL returns the download URL of a release asset by exact name, or "".
func assetURL(rel ghRelease, name string) string {
	for _, a := range rel.Assets {
		if a.Name == name {
			return a.URL
		}
	}
	return ""
}

func pickAsset(rel ghRelease, goos, goarch string) (name, url string) {
	for _, a := range rel.Assets {
		switch goos {
		case "windows":
			if strings.HasPrefix(a.Name, "Share2Us-Setup-") && strings.HasSuffix(a.Name, ".exe") {
				return a.Name, a.URL
			}
		case "darwin":
			if a.Name == "share2us-gui_darwin_universal.zip" {
				return a.Name, a.URL
			}
		default: // linux
			if a.Name == "share2us-gui_linux_"+goarch+".tar.gz" {
				return a.Name, a.URL
			}
		}
	}
	return "", ""
}
