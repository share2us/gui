package update

// Info is the result of an update check. It is shared by every build variant
// (the real updater and the Store stub), so it lives in an untagged file.
type Info struct {
	Available bool   `json:"available"`
	Current   string `json:"current"`
	Latest    string `json:"latest"`
	AssetURL  string `json:"assetUrl"`  // installer/archive for this OS ("" if none)
	AssetName string `json:"assetName"` // its filename
	Page      string `json:"page"`      // release page (fallback)
}
