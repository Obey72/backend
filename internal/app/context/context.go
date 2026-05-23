package context

import (
	"github.com/Obey72/backend/internal/app/response"
	"github.com/Obey72/backend/internal/roblox"
)

type Context struct {
	Client           *roblox.Client
	Pool             *roblox.ClientPool
	Logger           *logger
	PauseController  *pauseController
	CancelController *CancelController
	Response         *response.Response
}

// forceunblockpause is a thin shim that lets external packages (eg the http
// router) wake any goroutines parked in waitifpaused used in conjunction with
// cancelcontrollercancel to make cancellation immediate even mid-pause
func (c *Context) ForceUnblockPause() {
	if c.PauseController != nil {
		c.PauseController.ForceUnblock()
	}
}

func New(c *roblox.Client, resp *response.Response) *Context {
	return NewWithPool(c, nil, resp)
}

// newwithpool builds a context that has access to a pool of clients for
// quota rotation the single client (c) is still used for non-upload calls
// (asset info fetching permissions setup) since those operations don't
// burn quota and benefit from a stable identity the pool is consulted only
// for the actual sound upload call where quota matters
func NewWithPool(c *roblox.Client, pool *roblox.ClientPool, resp *response.Response) *Context {
	pc := newPauseController()
	cc := newCancelController()
	pc.linkCancel(cc)
	return &Context{
		Client:           c,
		Pool:             pool,
		Logger:           newLogger(),
		PauseController:  pc,
		CancelController: cc,
		Response:         resp,
	}
}
