package response

import (
	"encoding/json"
	"sync"
)

type ResponseItem struct {
	OldID int64 `json:"oldId"`
	NewID int64 `json:"newId"`
}

// skipreason categorizes why an asset was skipped during a job the desktop app
// uses these to print a useful end-of-job summary like:
//   skipped: 12 already owned 3 already accessible 2 wrong type
type SkipReason string

const (
	SkipAlreadyOwned      SkipReason = "already_owned"      // asset belongs to one of the user's accounts
	SkipAlreadyAccessible SkipReason = "already_accessible" // asset already has use permission for this place
	SkipWrongType         SkipReason = "wrong_type"         // filtered out by the type filter (rare plugin should pre-filter)
	SkipModerated         SkipReason = "moderated"          // asset is moderated and can't be re-downloaded
	SkipOther             SkipReason = "other"
)

// cache is the destructive queue drained by the studio plugin's get /
// history is a cumulative log used by the electron app's get /status never drained mid-job
// skipcounts tracks per-reason skip totals exposed via /status for end-of-job reporting
type Response struct {
	cache       []ResponseItem
	history     []ResponseItem
	skipCounts  map[SkipReason]int
	mutex       sync.RWMutex
	onItemAdded func(i ResponseItem)
	onSkipped   func(reason SkipReason, n int)
	onFailed    func(oldID int64, reason string)
}

func New(onItemAdded ...func(i ResponseItem)) *Response {
	var callback func(i ResponseItem)
	if len(onItemAdded) > 0 {
		callback = onItemAdded[0]
	}

	return &Response{
		cache:       make([]ResponseItem, 0),
		history:     make([]ResponseItem, 0),
		skipCounts:  make(map[SkipReason]int),
		onItemAdded: callback,
	}
}

// setonskipped wires a callback fired whenever addskipped records skipped items
// used by the http router to broadcast a "skip" sse event so the desktop app can
// log "skip n (reason)" lines mid-job instead of only seeing a totals line at the end
func (r *Response) SetOnSkipped(cb func(reason SkipReason, n int)) {
	r.mutex.Lock()
	r.onSkipped = cb
	r.mutex.Unlock()
}

// setonfailed wires a callback fired whenever addfailure records a per-id failure
// same purpose as setonskipped but for the bad path
func (r *Response) SetOnFailed(cb func(oldID int64, reason string)) {
	r.mutex.Lock()
	r.onFailed = cb
	r.mutex.Unlock()
}

// addfailure announces a per-id upload failure to the on-failed callback (if any)
// we don't keep history/counters here  assetdb already records the failure and the
// analytics endpoint tallies it this exists solely as a fan-out hook for the sse hub
func (r *Response) AddFailure(oldID int64, reason string) {
	r.mutex.RLock()
	cb := r.onFailed
	r.mutex.RUnlock()
	if cb != nil {
		go cb(oldID, reason)
	}
}

func (r *Response) AddItem(i ResponseItem) {
	r.mutex.Lock()

	r.cache = append(r.cache, i)
	r.history = append(r.history, i)
	if r.onItemAdded != nil {
		go r.onItemAdded(i)
	}

	r.mutex.Unlock()
}

func (r *Response) Clear() {
	r.mutex.Lock()
	r.cache = make([]ResponseItem, 0)
	r.mutex.Unlock()
}

// reset cumulative history called when a new job starts
func (r *Response) ClearHistory() {
	r.mutex.Lock()
	r.history = make([]ResponseItem, 0)
	r.skipCounts = make(map[SkipReason]int)
	r.mutex.Unlock()
}

// addskipped records that n items were skipped for the given reason called by
// the upload pipeline when filter rejects ids (already-owned already-accessible
// etc) the /status endpoint surfaces these counts so the desktop app can show
// "skipped: 12 already owned 3 already accessible" in the end-of-job log
func (r *Response) AddSkipped(reason SkipReason, n int) {
	if n <= 0 {
		return
	}
	r.mutex.Lock()
	r.skipCounts[reason] += n
	cb := r.onSkipped
	r.mutex.Unlock()
	if cb != nil {
		go cb(reason, n)
	}
}

// skippedcounts returns a snapshot of all skip counters map is a copy safe for
// the caller to retain empty map (not nil) when nothing was skipped
func (r *Response) SkippedCounts() map[string]int {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	out := make(map[string]int, len(r.skipCounts))
	for k, v := range r.skipCounts {
		out[string(k)] = v
	}
	return out
}

// skippedtotal returns the total number of skipped items across all reasons
func (r *Response) SkippedTotal() int {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	total := 0
	for _, v := range r.skipCounts {
		total += v
	}
	return total
}

func (r *Response) EncodeJSON(e *json.Encoder) error {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	return e.Encode(r.cache)
}

func (r *Response) Len() int {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	return len(r.cache)
}

// total items added since last clearhistory includes ones already drained from cache
func (r *Response) HistoryLen() int {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	return len(r.history)
}

// non-destructive peek at items added at or after history index idx
// used by the app for monitoring without competing with the plugin
func (r *Response) HistorySince(idx int) []ResponseItem {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	if idx < 0 {
		idx = 0
	}
	if idx >= len(r.history) {
		return []ResponseItem{}
	}
	out := make([]ResponseItem, len(r.history)-idx)
	copy(out, r.history[idx:])
	return out
}
