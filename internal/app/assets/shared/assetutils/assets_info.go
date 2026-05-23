package assetutils

import (
	"errors"
	"net"
	"time"

	"github.com/Obey72/backend/internal/app/assets/shared/clientutils"
	"github.com/Obey72/backend/internal/app/context"
	"github.com/Obey72/backend/internal/app/request"
	"github.com/Obey72/backend/internal/retry"
	"github.com/Obey72/backend/internal/roblox/develop"
	"github.com/Obey72/backend/internal/taskqueue"
)

const AssetsInfoChunkSize int = 50

type AssetsInfoResult = taskqueue.TaskResult[develop.GetAssetsInfoResponse]

// errcancelled is returned by the retry callback when the user issues a cancel
// during chunk fetching it uses exitretry so the retry loop returns immediately
// and the goroutine pool unblocks
var errCancelled = errors.New("job cancelled")

func GetAssetsInfoInChunks(ctx *context.Context, r *request.Request) []chan AssetsInfoResult {
	queue := taskqueue.New[develop.GetAssetsInfoResponse](time.Minute, 100)

	newAssetsInfoHandler := func(ids []int64) func() (develop.GetAssetsInfoResponse, error) {
		return func() (develop.GetAssetsInfoResponse, error) {
			handler, err := develop.NewAssetsInfoHandler(ctx.Client, ids)
			if err != nil {
				return develop.GetAssetsInfoResponse{}, err
			}

			return retry.Do(
				retry.NewOptions(retry.Tries(3)),
				func(try int) (develop.GetAssetsInfoResponse, error) {
					// surface cancel before any blocking call
					// retrydo treats exitretry as a clean exit so the chunk task returns
					// quickly and batchprocess sees the error in reserror
					if ctx.CancelController != nil && ctx.CancelController.IsCancelled() {
						return develop.GetAssetsInfoResponse{}, &retry.ExitRetry{Err: errCancelled}
					}
					ctx.PauseController.WaitIfPaused()
					// recheck after wait cancel may have been issued while we were paused
					if ctx.CancelController != nil && ctx.CancelController.IsCancelled() {
						return develop.GetAssetsInfoResponse{}, &retry.ExitRetry{Err: errCancelled}
					}
					if try > 1 {
						queue.Limiter.Wait()
					}

					assetsInfo, err := handler()
					if err == nil {
						return assetsInfo, nil
					}

					if err == develop.GetAssetsInfoErrors.ErrUnauthorized {
						clientutils.GetNewCookie(ctx, r, "cookie expired")
					} else {
						switch err.(type) {
						case *net.OpError, *net.DNSError:
							queue.Limiter.Decrement()
						}
					}

					return develop.GetAssetsInfoResponse{}, &retry.ContinueRetry{Err: err}
				},
			)
		}
	}

	ids := r.IDs

	chunkAmount := (len(ids) + AssetsInfoChunkSize - 1) / AssetsInfoChunkSize
	tasks := make([]chan AssetsInfoResult, 0, chunkAmount)
	for start, end := 0, 50; start < len(ids); start, end = start+50, end+50 {
		end = min(end, len(ids))
		idChunk := ids[start:end]
		tasks = append(tasks, queue.QueueTask(newAssetsInfoHandler(idChunk)))
	}

	return tasks
}
