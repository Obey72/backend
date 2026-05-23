package assets

import (
	"errors"
	"fmt"

	"github.com/Obey72/backend/internal/app/assets/animation"
	"github.com/Obey72/backend/internal/app/assets/mesh"
	"github.com/Obey72/backend/internal/app/assets/shared/clientutils"
	"github.com/Obey72/backend/internal/app/assets/shared/permissions"
	"github.com/Obey72/backend/internal/app/assets/sound"
	"github.com/Obey72/backend/internal/app/context"
	"github.com/Obey72/backend/internal/app/request"
	"github.com/Obey72/backend/internal/app/response"
	"github.com/Obey72/backend/internal/console"
	"github.com/Obey72/backend/internal/roblox"
)

var assetModules = map[string]func(ctx *context.Context, r *request.Request){
	"Animation": animation.Reupload,
	"Mesh":      mesh.Reupload,
	"Sound":     sound.Reupload,
}

// oncontextready fires when the upload pipeline has constructed its context
// the router stashes the cancel/pause controllers off ctx so /cancel can reach them
type OnContextReady func(ctx *context.Context)

// newreuploadhandlerwithtype returns a handler that runs the upload pipeline
// onready (optional) is called with the freshly-built context before uploads start
// giving the caller a chance to grab references to cancel/pause controllers
//
// backwards compatible single-client variant for multi-cookie/pool support
// use newreuploadhandlerwithpool
func NewReuploadHandlerWithType(
	assetType string,
	c *roblox.Client,
	r *request.RawRequest,
	resp *response.Response,
	onReady ...OnContextReady,
) (func() error, error) {
	return NewReuploadHandlerWithPool(assetType, c, nil, r, resp, onReady...)
}

// newreuploadhandlerwithpool is the pool-aware variant when pool is non-nil
// and has 2+ entries sound uploads will rotate between accounts on quota/auth
// errors and grant holder permission on each upload when pool is nil behavior
// is identical to single-client mode
func NewReuploadHandlerWithPool(
	assetType string,
	c *roblox.Client,
	pool *roblox.ClientPool,
	r *request.RawRequest,
	resp *response.Response,
	onReady ...OnContextReady,
) (func() error, error) {
	reupload, exists := assetModules[assetType]
	if !exists {
		return func() error { return nil }, errors.New(assetType + " module does not exist")
	}

	return func() error {
		ctx := context.NewWithPool(c, pool, resp)

		// publish cancel/pause refs to whoever needs them (eg http /cancel handler)
		// must happen before any actual work so cancellation is responsive even during setup
		for _, hook := range onReady {
			if hook != nil {
				hook(ctx)
			}
		}

		console.ClearScreen()

		fmt.Println("Getting current place details...")
		req, err := request.FromRawRequest(c, r)
		console.ClearScreen()
		if err != nil {
			return err
		}

		fmt.Println("Checking if account can edit universe...")
		err = permissions.CanEditUniverse(ctx, req)
		console.ClearScreen()
		if err != nil {
			clientutils.GetNewCookie(ctx, req, err.Error())
		}

		reupload(ctx, req)
		return nil
	}, nil
}

func DoesModuleExist(m string) bool {
	_, exists := assetModules[m]
	return exists
}
