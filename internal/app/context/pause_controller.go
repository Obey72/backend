package context

import "sync"

// pausecontroller gates work-in-progress originally only supported pause/unpause
// for cookie refresh flows extended here to also short-circuit on cancellation
// so an aborted pipeline doesn't keep churning through queued retries
type pauseController struct {
	IsPaused bool
	mutex    sync.RWMutex
	signal   chan struct{}

	// cancelref is set by contextnew to point at the same cancelcontroller
	// the http layer triggers waitifpaused checks this before blocking and
	// returns immediately if a cancel has been issued
	cancelRef *CancelController
}

func newPauseController() *pauseController {
	signal := make(chan struct{})
	close(signal)

	return &pauseController{
		signal: signal,
	}
}

// linkcancel wires the pause controller to a cancel controller so paused
// goroutines can be unblocked by a cancel without needing an unpause call
func (c *pauseController) linkCancel(cc *CancelController) {
	c.mutex.Lock()
	c.cancelRef = cc
	c.mutex.Unlock()
}

func (c *pauseController) WaitIfPaused() {
	c.mutex.RLock()
	signal := c.signal
	cancel := c.cancelRef
	c.mutex.RUnlock()

	if cancel != nil && cancel.IsCancelled() {
		return
	}

	// race-safe wait: if a cancel arrives while we're blocked here the http
	// layer will close(signal) via unpause-on-cancel so we wake up the caller
	// then re-checks iscancelled and exits
	<-signal
}

func (c *pauseController) Pause() (success bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.IsPaused {
		return false
	}

	c.signal = make(chan struct{})
	c.IsPaused = true
	return true
}

func (c *pauseController) Unpause() (success bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if !c.IsPaused {
		return false
	}

	close(c.signal)
	c.IsPaused = false
	return true
}

// forceunblock wakes any goroutines parked in waitifpaused without changing
// the ispaused flag used by cancel to release everyone immediately so they
// can hit the iscancelled check and bail out
func (c *pauseController) ForceUnblock() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.IsPaused {
		close(c.signal)
		c.IsPaused = false
	}
}
