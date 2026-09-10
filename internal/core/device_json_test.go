package core

import (
	"encoding/json"
	"sort"
	"testing"
)

// The frontend declares this shape in frontend/src/main.ts:
//
//	type CloudDevice = { sessionId; name; label; publicKey; hasKey; current }
//
// Nothing else checks that Go actually produces it. The frontend's own tests
// mock the binding with the keys it WANTS, so they stayed green for as long as
// the real backend sent something different -- which it did, and the device list
// hung on "Looking for your devices…" as a result.
func TestDeviceJSONMatchesTheFrontendContract(t *testing.T) {
	raw, err := json.Marshal(Device{})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	want := []string{"current", "hasKey", "label", "name", "publicKey", "sessionId"}
	keys := make([]string, 0, len(got))
	for k := range got {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	if len(keys) != len(want) {
		t.Fatalf("keys = %v, want %v", keys, want)
	}
	for i, k := range want {
		if keys[i] != k {
			t.Fatalf("keys = %v, want %v (the frontend reads these names)", keys, want)
		}
	}
}
