package update

import "strings"

// thumbprintAccepted reports whether an update signed by cert `actual` is
// acceptable to a build pinned to `pinned`.
//
// An EMPTY pin means this build was produced without a stable signing
// certificate, so there is nothing to compare against and the checksum is the
// only integrity gate. It must never be read as "accept anything": that
// distinction is the whole point of the pin, so it is kept in one tested place
// rather than inline in a Windows-only file that no test runs.
func thumbprintAccepted(pinned, actual string) bool {
	pinned = strings.TrimSpace(pinned)
	if pinned == "" {
		return true // unpinned build: not verified here, verified by checksum
	}
	return strings.EqualFold(strings.TrimSpace(actual), pinned)
}
