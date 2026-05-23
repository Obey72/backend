package roblox

import (
	"errors"
	"strings"
	"sync"
	"time"
)

type ClientPool struct {
	mu        sync.RWMutex
	entries   []*poolEntry
	cursor    int
	holderIDs []string
}

type poolEntry struct {
	client    *Client
	apiKey    string
	exhausted bool
	invalid   bool
	// cooldown lets the pool temporarily skip a client (eg rate limited)
	// without marking it permanently invalid zero value = no cooldown
	cooluntil time.Time
	// speed tracking: rolling average of ms per upload
	uploadCount int64
	totalMs     int64
}

type PoolStats struct {
	Total     int
	Active    int
	Exhausted int
	Invalid   int
	// per-slot health for the frontend
	Slots []PoolSlot
}

type PoolSlot struct {
	UserID    int64   `json:"userId"`
	Active    bool    `json:"active"`
	Exhausted bool    `json:"exhausted"`
	Invalid   bool    `json:"invalid"`
	Cooldown  bool    `json:"cooldown"`
	AvgMs     float64 `json:"avgMs"` // average ms per upload 0 if unknown
}

var (
	ErrPoolEmpty    = errors.New("client pool is empty")
	ErrAllExhausted = errors.New("all clients exhausted or invalid")
)

func NewPool() *ClientPool {
	return &ClientPool{
		entries: make([]*poolEntry, 0),
	}
}

func (p *ClientPool) Replace(clients []*Client, apiKeys []string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.entries = make([]*poolEntry, 0, len(clients))
	for i, c := range clients {
		key := ""
		if i < len(apiKeys) {
			key = apiKeys[i]
		}
		p.entries = append(p.entries, &poolEntry{
			client:  c,
			apiKey:  key,
			invalid: c == nil || c.UserInfo.ID == 0,
		})
	}
	p.cursor = 0
}

func (p *ClientPool) Acquire() (*Client, string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.entries) == 0 {
		return nil, "", ErrPoolEmpty
	}
	now := time.Now()
	for i := 0; i < len(p.entries); i++ {
		idx := (p.cursor + i) % len(p.entries)
		e := p.entries[idx]
		if e.exhausted || e.invalid || e.client == nil {
			continue
		}
		if !e.cooluntil.IsZero() && now.Before(e.cooluntil) {
			continue
		}
		p.cursor = (idx + 1) % len(p.entries)
		return e.client, e.apiKey, nil
	}
	return nil, "", ErrAllExhausted
}

func (p *ClientPool) MarkExhausted(c *Client) {
	if c == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, e := range p.entries {
		if e.client == c {
			e.exhausted = true
			return
		}
	}
}

func (p *ClientPool) MarkInvalid(c *Client) {
	if c == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, e := range p.entries {
		if e.client == c {
			e.invalid = true
			return
		}
	}
}

// cooldown temporarily disables a client for the given duration
// used for rate-limited clients that aren't fully invalid just slow
func (p *ClientPool) Cooldown(c *Client, d time.Duration) {
	if c == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, e := range p.entries {
		if e.client == c {
			e.cooluntil = time.Now().Add(d)
			return
		}
	}
}

// recordupload tracks upload speed for a client slot
// elapsedms is the time the upload took end-to-end
func (p *ClientPool) RecordUpload(c *Client, elapsedMs int64) {
	if c == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, e := range p.entries {
		if e.client == c {
			e.uploadCount++
			e.totalMs += elapsedMs
			return
		}
	}
}

func (p *ClientPool) Stats() PoolStats {
	p.mu.RLock()
	defer p.mu.RUnlock()
	now := time.Now()
	s := PoolStats{
		Total: len(p.entries),
		Slots: make([]PoolSlot, 0, len(p.entries)),
	}
	for _, e := range p.entries {
		oncooldown := !e.cooluntil.IsZero() && now.Before(e.cooluntil)
		isactive := !e.invalid && !e.exhausted && !oncooldown && e.client != nil
		switch {
		case e.invalid:
			s.Invalid++
		case e.exhausted:
			s.Exhausted++
		default:
			if !oncooldown {
				s.Active++
			}
		}
		var avgms float64
		if e.uploadCount > 0 {
			avgms = float64(e.totalMs) / float64(e.uploadCount)
		}
		var uid int64
		if e.client != nil {
			uid = e.client.UserInfo.ID
		}
		s.Slots = append(s.Slots, PoolSlot{
			UserID:    uid,
			Active:    isactive,
			Exhausted: e.exhausted,
			Invalid:   e.invalid,
			Cooldown:  oncooldown,
			AvgMs:     avgms,
		})
	}
	return s
}

func (p *ClientPool) HasAny() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.entries) > 0
}

// setholderids replaces the holder list duplicates and empty strings are
// dropped so callers can pass raw input without scrubbing it first
func (p *ClientPool) SetHolderIDs(ids []string) {
	cleaned := make([]string, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" || id == "0" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		cleaned = append(cleaned, id)
	}
	p.mu.Lock()
	p.holderIDs = cleaned
	p.mu.Unlock()
}

// setholderid is kept as a single-value convenience used by older callers and
// tests delegates to setholderids so the dedup/clean logic lives in one place
func (p *ClientPool) SetHolderID(id string) {
	p.SetHolderIDs([]string{id})
}

func (p *ClientPool) HolderIDs() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]string, len(p.holderIDs))
	copy(out, p.holderIDs)
	return out
}

// holderid returns the first holder if any kept for callers that only
// understand a single holder mainly the legacy status payload
func (p *ClientPool) HolderID() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if len(p.holderIDs) == 0 {
		return ""
	}
	return p.holderIDs[0]
}

func (p *ClientPool) FindByCookie(cookie string) *Client {
	p.mu.RLock()
	defer p.mu.RUnlock()
	for _, e := range p.entries {
		if e.client != nil && e.client.Cookie == cookie {
			return e.client
		}
	}
	return nil
}

