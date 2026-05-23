package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const filename = "upload_session.json"

type UploadedEntry struct {
	OldID int64  `json:"oldId"`
	NewID int64  `json:"newId"`
	Time  int64  `json:"time"`
}

type FailedEntry struct {
	ID      int64  `json:"id"`
	Reason  string `json:"reason"`
	Retries int    `json:"retries"`
}

type Session struct {
	ID          string          `json:"id"`
	AssetType   string          `json:"assetType"`
	PendingIDs  []int64         `json:"pendingIds"`
	Uploaded    []UploadedEntry `json:"uploaded"`
	Failed      []FailedEntry   `json:"failed"`
	StartedAt   int64           `json:"startedAt"`
	UpdatedAt   int64           `json:"updatedAt"`
	PlaceID     int64           `json:"placeId"`
	UniverseID  int64           `json:"universeId"`
}

var (
	mu      sync.Mutex
	datadir string
)

func Init(dir string) {
	mu.Lock()
	datadir = dir
	mu.Unlock()
}

func sessionpath() string {
	return filepath.Join(datadir, filename)
}

func Load() (*Session, error) {
	mu.Lock()
	defer mu.Unlock()
	data, err := os.ReadFile(sessionpath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func Save(s *Session) error {
	mu.Lock()
	defer mu.Unlock()
	s.UpdatedAt = time.Now().UnixMilli()
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return os.WriteFile(sessionpath(), data, 0644)
}

func Delete() error {
	mu.Lock()
	defer mu.Unlock()
	err := os.Remove(sessionpath())
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func Exists() bool {
	mu.Lock()
	defer mu.Unlock()
	_, err := os.Stat(sessionpath())
	return err == nil
}
