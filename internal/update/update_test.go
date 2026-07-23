package update

import "testing"

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
