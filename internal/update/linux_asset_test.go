package update

import "testing"

// Linux amd64 ships two builds compiled against mutually incompatible
// webkit2gtk generations. Ubuntu dropped libwebkit2gtk-4.0 after 22.04 and it is
// not installable on 24.04, so offering the wrong one is not a degraded update
// -- it is an update to a binary that will not start.
//
// Found 2026-09-17 by downloading the shipped v20260916200055 tarball onto
// Ubuntu 24.04: libwebkit2gtk-4.0.so.37 => not found.

func withWebkit41(t *testing.T, present bool) {
	t.Helper()
	prev := webkit41Present
	webkit41Present = func() bool { return present }
	t.Cleanup(func() { webkit41Present = prev })
}

func TestLinuxAssetNameFollowsTheInstalledWebkit(t *testing.T) {
	withWebkit41(t, true)
	if got := linuxAssetName("linux", "amd64"); got != "share2us-gui_linux_amd64_webkit41.tar.gz" {
		t.Errorf("with webkit 4.1 installed, asset = %q, want the webkit41 build", got)
	}

	withWebkit41(t, false)
	if got := linuxAssetName("linux", "amd64"); got != "share2us-gui_linux_amd64.tar.gz" {
		t.Errorf("without webkit 4.1, asset = %q, want the 4.0 build", got)
	}
}

// arm64 has one build and must keep its existing name whatever is installed:
// there is no 22.04 arm64 runner and so no second asset to choose.
func TestArm64IsUnaffectedByTheProbe(t *testing.T) {
	for _, present := range []bool{true, false} {
		withWebkit41(t, present)
		if got := linuxAssetName("linux", "arm64"); got != "share2us-gui_linux_arm64.tar.gz" {
			t.Errorf("webkit41=%v: arm64 asset = %q, want the unsuffixed name", present, got)
		}
	}
}

// The 4.0 asset keeps the name every already-installed client matches on. If
// this ever changes, those clients stop finding any asset and silently stop
// updating -- the failure is invisible from the server side.
func TestExistingClientsStillFindTheirAsset(t *testing.T) {
	withWebkit41(t, false)
	rel := ghRelease{Assets: []ghAsset{
		{Name: "share2us-gui_linux_amd64_webkit41.tar.gz", URL: "u-41"},
		{Name: "share2us-gui_linux_amd64.tar.gz", URL: "u-40"},
	}}
	name, url := pickAsset(rel, "linux", "amd64")
	if name != "share2us-gui_linux_amd64.tar.gz" || url != "u-40" {
		t.Fatalf("picked %q/%q, want the 4.0 asset", name, url)
	}
}

// And with 4.1 present the newer asset is chosen even though the 4.0 one is
// listed first -- selection must be by name, not by order in the release.
func TestPickAssetPrefersTheMatchingBuildNotTheFirstOne(t *testing.T) {
	withWebkit41(t, true)
	rel := ghRelease{Assets: []ghAsset{
		{Name: "share2us-gui_linux_amd64.tar.gz", URL: "u-40"},
		{Name: "share2us-gui_linux_amd64_webkit41.tar.gz", URL: "u-41"},
	}}
	name, url := pickAsset(rel, "linux", "amd64")
	if name != "share2us-gui_linux_amd64_webkit41.tar.gz" || url != "u-41" {
		t.Fatalf("picked %q/%q, want the webkit41 asset", name, url)
	}
}

// A release cut before the webkit41 build existed has only the 4.0 asset. A
// 24.04 machine must still be offered something rather than nothing: it cannot
// run it, but returning "" reads as "no update available", which is a worse lie
// than offering the only build that exists.
func TestFallsBackWhenOnlyTheOldAssetExists(t *testing.T) {
	withWebkit41(t, true)
	rel := ghRelease{Assets: []ghAsset{{Name: "share2us-gui_linux_amd64.tar.gz", URL: "u-40"}}}
	name, url := pickAsset(rel, "linux", "amd64")
	if name != "share2us-gui_linux_amd64.tar.gz" || url != "u-40" {
		t.Fatalf("picked %q/%q, want a fallback to the only asset present -- returning nothing reads as \"up to date\"", name, url)
	}
}
