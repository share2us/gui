package update

import "strings"

// Info is the result of an update check. It is shared by every build variant
// (the real updater and the Store stub), so it lives in an untagged file.
type Info struct {
	Available bool   `json:"available"`
	Current   string `json:"current"`
	Latest    string `json:"latest"`
	AssetURL  string `json:"assetUrl"`  // installer/archive for this OS ("" if none)
	AssetName string `json:"assetName"` // its filename
	// SHA256URL is the published .sha256 sibling of AssetURL ("" when the release
	// has none). The updater verifies the download against it before running
	// anything — see VerifyChecksum.
	SHA256URL string `json:"sha256Url"`
	Page      string `json:"page"` // release page (fallback)
	// Channel is the channel that was checked ("stable" or "beta"); Prerelease is
	// true when the offered build is a GitHub pre-release (beta channel only).
	Channel    string `json:"channel"`
	Prerelease bool   `json:"prerelease"`
}

// Release channels. Stable resolves GitHub's "latest" release, which is never a
// pre-release, so betas are invisible to stable installs by construction. Beta
// resolves the newest non-draft release, pre-releases included; a stable that is
// newer than the last beta is offered to beta installs too.
const (
	ChannelStable = "stable"
	ChannelBeta   = "beta"
)

// NormalizeChannel maps "", "stable" and "beta" (any case) to a channel constant;
// anything else is treated as stable so a bad saved value never breaks the check.
func NormalizeChannel(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case ChannelBeta:
		return ChannelBeta
	default:
		return ChannelStable
	}
}
