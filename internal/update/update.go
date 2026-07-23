// Package update checks GitHub Releases for a newer Share2Us build. Versions are
// UTC-timestamp strings (main.buildVersion), so "newer" is a plain string compare
// of the release tag against the running build.
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

const defaultReleasesURL = "https://api.github.com/repos/share2us/gui/releases/latest"

// Info is the result of an update check.
type Info struct {
	Available bool   `json:"available"`
	Current   string `json:"current"`
	Latest    string `json:"latest"`
	AssetURL  string `json:"assetUrl"`  // installer/archive for this OS ("" if none)
	AssetName string `json:"assetName"` // its filename
	Page      string `json:"page"`      // release page (fallback)
}

type ghAsset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

type ghRelease struct {
	TagName string    `json:"tag_name"`
	HTMLURL string    `json:"html_url"`
	Assets  []ghAsset `json:"assets"`
}

// Check queries the latest release for the running OS/arch.
func Check(ctx context.Context, current string) (Info, error) {
	client := &http.Client{Timeout: 12 * time.Second}
	return checkAt(ctx, client, defaultReleasesURL, current, runtime.GOOS, runtime.GOARCH)
}

func checkAt(ctx context.Context, client *http.Client, url, current, goos, goarch string) (Info, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Info{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "share2us-gui")
	resp, err := client.Do(req)
	if err != nil {
		return Info{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Info{}, fmt.Errorf("update check: HTTP %d", resp.StatusCode)
	}
	var rel ghRelease
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&rel); err != nil {
		return Info{}, err
	}
	return infoFrom(rel, current, goos, goarch), nil
}

func infoFrom(rel ghRelease, current, goos, goarch string) Info {
	latest := strings.TrimPrefix(strings.TrimSpace(rel.TagName), "v")
	info := Info{Current: current, Latest: latest, Page: rel.HTMLURL}
	// A dev/unstamped build never nags; otherwise newer = lexicographically greater
	// (timestamps are fixed-width, so this equals numeric order).
	info.Available = current != "" && current != "dev" && latest != "" && latest > current
	info.AssetName, info.AssetURL = pickAsset(rel, goos, goarch)
	return info
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
