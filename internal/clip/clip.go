// Package clip reads shareable content (image or text) directly from the OS
// clipboard so the app can offer a one-click "share what you just copied"
// suggestion. The real implementation is Windows-only (where it needs no CGO);
// other platforms fall back to the browser Ctrl+V paste path and report nothing.
package clip

import "strings"

// Suggestion describes shareable clipboard content for a UI chip. Kind is
// "image", "text", or "none" when there is nothing shareable.
type Suggestion struct {
	Kind    string `json:"kind"`
	Preview string `json:"preview"` // short human preview (a text snippet or a label)
	Ext     string `json:"ext"`     // file extension without the dot (png/txt/md)
}

// snippet renders a single-line, length-capped preview of copied text.
func snippet(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	r := []rune(s)
	if len(r) > 60 {
		return string(r[:57]) + "…"
	}
	return s
}

// extForText picks md for markdown-looking text, else txt.
func extForText(s string) string {
	for _, line := range strings.Split(s, "\n") {
		l := strings.TrimSpace(line)
		if strings.HasPrefix(l, "#") || strings.HasPrefix(l, "- ") ||
			strings.HasPrefix(l, "* ") || strings.HasPrefix(l, "> ") ||
			strings.HasPrefix(l, "```") {
			return "md"
		}
	}
	if strings.Contains(s, "](") {
		return "md"
	}
	return "txt"
}
