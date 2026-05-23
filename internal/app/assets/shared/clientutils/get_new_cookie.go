package clientutils

import (
	"errors"
	"fmt"
	"time"

	"github.com/Obey72/backend/internal/app/assets/shared/permissions"
	"github.com/Obey72/backend/internal/app/config"
	"github.com/Obey72/backend/internal/app/context"
	"github.com/Obey72/backend/internal/app/request"
	"github.com/Obey72/backend/internal/color"
	"github.com/Obey72/backend/internal/console"
	"github.com/Obey72/backend/internal/files"
)

var cookieFile = config.Get("cookie_file")

// getnewcookie prompts the user via stdin to enter a new roblosecurity when the
// current one expires when run as a child process spawned by the electron app
// stdin has no human attached so this would block forever we now bail out
// immediately on cancellation and also poll for cancel during the retry loop so
// the user can recover via the desktop app's cancel button
func GetNewCookie(ctx *context.Context, r *request.Request, m string) {
	pauseController := ctx.PauseController
	cancelController := ctx.CancelController

	// if a cancel was already issued don't even try to prompt
	if cancelController != nil && cancelController.IsCancelled() {
		return
	}

	if !pauseController.Pause() {
		pauseController.WaitIfPaused()
		return
	}

	// longinput blocks on stdin which doesn't exist in the electron-spawned process
	// detect this and just bail with the cancel flag flipped so the goroutines exit
	// gracefully instead of deadlocking callers see waitifpaused return because
	// of forceunblock
	if !console.IsInteractive() {
		// no terminal attached nothing useful we can do here
		// flip cancel so the upload loop exits cleanly
		if cancelController != nil {
			cancelController.Cancel(false)
		}
		pauseController.Unpause()
		return
	}

	console.ClearScreen()

	client := ctx.Client
	inputErr := errors.New(m)
	for {
		// surface cancel between retries so user can abort from the desktop app
		if cancelController != nil && cancelController.IsCancelled() {
			pauseController.Unpause()
			return
		}

		fmt.Print(ctx.Logger.History.String())
		color.Error.Println(inputErr)

		i, err := console.LongInput("ROBLOSECURITY: ")
		console.ClearScreen()
		if err != nil {
			inputErr = err
			// brief sleep so the loop doesn't hot-spin when stdin is closed
			time.Sleep(500 * time.Millisecond)
			continue
		}

		fmt.Println("Authenticating cookie...")
		err = client.SetCookie(i)
		console.ClearScreen()
		if err != nil {
			inputErr = err
			continue
		}

		fmt.Println("Checking if account can edit universe...")
		err = permissions.CanEditUniverse(ctx, r)
		console.ClearScreen()
		if err != nil {
			inputErr = err
			continue
		}

		break
	}

	fmt.Print(ctx.Logger.History.String())

	if err := files.Write(cookieFile, client.Cookie); err != nil {
		ctx.Logger.Error("Failed to save cookie: ", err)
	}

	pauseController.Unpause()
}
