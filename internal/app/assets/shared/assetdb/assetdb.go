package assetdb

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const filename = "upload_history.json"

type Entry struct {
	OldID     int64  `json:"oldId"`
	NewID     int64  `json:"newId"`
	AssetType string `json:"assetType"`
	Status    string `json:"status"` // "ok" | "failed"
	Error     string `json:"error,omitempty"`
	Cookie    string `json:"cookie,omitempty"` // last 8 chars only for identification
	Time      int64  `json:"time"`
}

type DB struct {
	mu      sync.Mutex
	entries []Entry
	path    string
}

var global *DB

func Init(dir string) {
	global = &DB{
		path: filepath.Join(dir, filename),
	}
	global.load()
}

func (db *DB) load() {
	data, err := os.ReadFile(db.path)
	if err != nil {
		db.entries = make([]Entry, 0)
		return
	}
	json.Unmarshal(data, &db.entries)
}

func (db *DB) flush() {
	data, err := json.Marshal(db.entries)
	if err != nil {
		return
	}
	os.WriteFile(db.path, data, 0644)
}

func (db *DB) add(e Entry) {
	db.mu.Lock()
	defer db.mu.Unlock()
	e.Time = time.Now().UnixMilli()
	db.entries = append(db.entries, e)
	// keep last 10000 entries
	if len(db.entries) > 10000 {
		db.entries = db.entries[len(db.entries)-10000:]
	}
	db.flush()
}

func RecordSuccess(oldID, newID int64, assetType, cookieSuffix string) {
	if global == nil {
		return
	}
	global.add(Entry{
		OldID: oldID, NewID: newID,
		AssetType: assetType,
		Status:    "ok",
		Cookie:    cookieSuffix,
	})
}

func RecordFailure(oldID int64, assetType, reason, cookieSuffix string) {
	if global == nil {
		return
	}
	global.add(Entry{
		OldID:     oldID,
		AssetType: assetType,
		Status:    "failed",
		Error:     reason,
		Cookie:    cookieSuffix,
	})
}

func Recent(n int) []Entry {
	if global == nil {
		return nil
	}
	global.mu.Lock()
	defer global.mu.Unlock()
	entries := global.entries
	if n <= 0 || n >= len(entries) {
		out := make([]Entry, len(entries))
		copy(out, entries)
		return out
	}
	out := make([]Entry, n)
	copy(out, entries[len(entries)-n:])
	return out
}

func Stats() (total, succeeded, failed int) {
	if global == nil {
		return
	}
	global.mu.Lock()
	defer global.mu.Unlock()
	total = len(global.entries)
	for _, e := range global.entries {
		if e.Status == "ok" {
			succeeded++
		} else {
			failed++
		}
	}
	return
}
