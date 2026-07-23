package lanid

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ---- broadcast-discovery scan interval ----

const defaultScanIntervalSec = 60

type settingsFile struct {
	ScanIntervalSec int `json:"scanIntervalSec"`
}

var setMu sync.Mutex

func settingsPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "lan_settings.json"), nil
}

// GetScanInterval returns the broadcast-discovery scan interval in seconds
// (default 60; 0 = manual refresh only).
func GetScanInterval() int {
	p, err := settingsPath()
	if err != nil {
		return defaultScanIntervalSec
	}
	setMu.Lock()
	defer setMu.Unlock()
	data, err := os.ReadFile(p)
	if err != nil {
		return defaultScanIntervalSec
	}
	var s settingsFile
	if json.Unmarshal(data, &s) != nil || s.ScanIntervalSec < 0 {
		return defaultScanIntervalSec
	}
	return s.ScanIntervalSec
}

// SetScanInterval persists the scan interval (seconds; 0 = manual only).
func SetScanInterval(sec int) error {
	if sec < 0 {
		sec = 0
	}
	p, err := settingsPath()
	if err != nil {
		return err
	}
	setMu.Lock()
	defer setMu.Unlock()
	data, _ := json.MarshalIndent(settingsFile{ScanIntervalSec: sec}, "", "  ")
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

// ---- activity log (metadata only, newest first, capped) ----

const maxActivity = 200

// ActivityEntry is one logged transfer/broadcast event. Metadata only — never
// file contents.
type ActivityEntry struct {
	Kind string `json:"kind"` // sent | received | downloaded | broadcast
	Peer string `json:"peer"`
	Name string `json:"name"`
	Size int64  `json:"size"`
	TS   int64  `json:"ts"` // unix seconds
}

var actMu sync.Mutex

func activityPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "lan_activity.json"), nil
}

// ActivityAppend records an event, keeping the newest maxActivity entries.
func ActivityAppend(e ActivityEntry) {
	p, err := activityPath()
	if err != nil {
		return
	}
	if e.TS == 0 {
		e.TS = time.Now().Unix()
	}
	actMu.Lock()
	defer actMu.Unlock()
	list := append([]ActivityEntry{e}, readActivityLocked(p)...) // newest first
	if len(list) > maxActivity {
		list = list[:maxActivity]
	}
	writeActivityLocked(p, list)
}

// ActivityList returns the log, newest first.
func ActivityList() []ActivityEntry {
	p, err := activityPath()
	if err != nil {
		return nil
	}
	actMu.Lock()
	defer actMu.Unlock()
	return readActivityLocked(p)
}

// ActivityClear empties the log.
func ActivityClear() {
	if p, err := activityPath(); err == nil {
		actMu.Lock()
		_ = os.Remove(p)
		actMu.Unlock()
	}
}

func readActivityLocked(p string) []ActivityEntry {
	data, err := os.ReadFile(p)
	if err != nil {
		return nil
	}
	var list []ActivityEntry
	if json.Unmarshal(data, &list) != nil {
		return nil
	}
	return list
}

func writeActivityLocked(p string, list []ActivityEntry) {
	data, _ := json.MarshalIndent(list, "", "  ")
	tmp := p + ".tmp"
	if os.WriteFile(tmp, data, 0o600) == nil {
		_ = os.Rename(tmp, p)
	}
}
