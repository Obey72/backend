package context

import "sync"

// cancelcontroller lets the http layer signal a running upload pipeline to abort
// two cancel modes: keepresults preserves what's already been uploaded so the
// plugin can still replace those ids discard tells the plugin to skip replacement
// the pipeline polls iscancelled at safe points (between retries before next batch)
// and exits gracefully without partial-state corruption
//
// cancellation can be initiated by either the user (via /cancel http endpoint) or
// internally by the pipeline itself (eg quota exceeded no point retrying)
// reason() carries an optional human-readable cause that the http /status surfaces
// to the desktop app and plugin
type CancelController struct {
	mu        sync.RWMutex
	cancelled bool
	keep      bool
	reason    string
}

func newCancelController() *CancelController {
	return &CancelController{}
}

// cancel marks the pipeline as aborted keep=true keeps already-uploaded ids
// so the plugin still gets to replace them keep=false signals the plugin to
// throw away its replacement map entirely
func (c *CancelController) Cancel(keep bool) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cancelled {
		return false
	}
	c.cancelled = true
	c.keep = keep
	return true
}

// cancelwithreason is like cancel but attaches a human-readable explanation
// that the http layer can surface to the user
func (c *CancelController) CancelWithReason(keep bool, reason string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cancelled {
		return false
	}
	c.cancelled = true
	c.keep = keep
	c.reason = reason
	return true
}

// reset is called when a new upload job starts clears any prior cancel state
func (c *CancelController) Reset() {
	c.mu.Lock()
	c.cancelled = false
	c.keep = false
	c.reason = ""
	c.mu.Unlock()
}

func (c *CancelController) IsCancelled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.cancelled
}

// keepresults reports whether a cancel was issued with keep=true only meaningful
// after iscancelled() returns true
func (c *CancelController) KeepResults() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.keep
}

// reason returns the optional human-readable cause for cancellation
// empty when no reason was set
func (c *CancelController) Reason() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.reason
}
