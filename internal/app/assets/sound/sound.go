package sound

import (
	"errors"
	"fmt"
	"net"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Obey72/backend/internal/app/assets/shared/assetutils"
	"github.com/Obey72/backend/internal/app/assets/shared/clientutils"
	"github.com/Obey72/backend/internal/app/assets/shared/uploaderror"
	"github.com/Obey72/backend/internal/app/context"
	"github.com/Obey72/backend/internal/app/request"
	"github.com/Obey72/backend/internal/app/response"
	"github.com/Obey72/backend/internal/app/tuning"
	"github.com/Obey72/backend/internal/atomicarray"
	"github.com/Obey72/backend/internal/color"
	"github.com/Obey72/backend/internal/retry"
	"github.com/Obey72/backend/internal/roblox"
	"github.com/Obey72/backend/internal/roblox/assetdelivery"
	"github.com/Obey72/backend/internal/roblox/assets"
	"github.com/Obey72/backend/internal/roblox/develop"
	"github.com/Obey72/backend/internal/roblox/games"
	"github.com/Obey72/backend/internal/roblox/publish"
	"github.com/Obey72/backend/internal/shardedmap"
	"github.com/Obey72/backend/internal/taskqueue"
)

const assetTypeID int32 = 3

var ErrUnauthorized = errors.New("authentication required to access asset")

// grantholderpermission gives every holder user id use permission on a freshly
// uploaded asset best-effort: failures get logged but don't block the upload
// flow called after every successful upload when at least one holder is set
//
// xsrf token retry: roblox responds 403 + new x-csrf-token on first attempt to
// any state-changing call when no token is set on the client we retry once on
// errtokeninvalid because settoken updated the client's token in the failed
// request handler
func grantHolderPermission(c *roblox.Client, assetID int64, holderUserIDs []string, logErr func(args ...any)) {
	if c == nil || len(holderUserIDs) == 0 {
		return
	}

	requests := make([]assets.PermissionRequestItem, 0, len(holderUserIDs))
	for _, id := range holderUserIDs {
		if id == "" || id == "0" {
			continue
		}
		requests = append(requests, assets.PermissionRequestItem{
			SubjectType: "User", SubjectID: id, Action: "Use",
		})
	}
	if len(requests) == 0 {
		return
	}
	body := assets.PermissionRequest{Requests: requests}

	for try := 0; try < 2; try++ {
		handler, err := assets.NewUpdatePermissionsHandler(c, assetID, body)
		if err != nil {
			if logErr != nil {
				logErr(fmt.Sprintf("permission grant prep failed for asset %d: %v", assetID, err))
			}
			return
		}
		_, err = handler()
		if err == nil {
			return
		}
		if err == assets.UpdatePermissionErrors.ErrTokenInvalid && try == 0 {
			// token got refreshed by the failed call retry once
			continue
		}
		if logErr != nil {
			logErr(fmt.Sprintf("permission grant failed for asset %d: %v", assetID, err))
		}
		return
	}
}

func MoveValueToTop[T comparable](arr *atomicarray.AtomicArray[T], value T) {
	arr.Update(func(currentArray []T) []T {
		if currentArray[0] == value {
			return nil
		}

		for i, v := range currentArray {
			if v != value {
				continue
			}
			if i == 1 {
				currentArray[0], currentArray[1] = currentArray[1], currentArray[0]
				return currentArray
			}

			copy(currentArray[1:i+1], currentArray[0:i])
			currentArray[0] = value
			return currentArray
		}

		return nil
	})
}

func Reupload(ctx *context.Context, r *request.Request) {
	client := ctx.Client
	logger := ctx.Logger
	pauseController := ctx.PauseController
	cancelController := ctx.CancelController
	resp := ctx.Response

	idsToUpload := len(r.IDs)
	var idsProcessed atomic.Int32

	defaultPlaceIDs := r.DefaultPlaceIDs
	defaultPlaceIDsMap := make(map[int64]struct{}, len(defaultPlaceIDs))
	for _, placeID := range defaultPlaceIDs {
		defaultPlaceIDsMap[placeID] = struct{}{}
	}

	var groupID int64
	if r.IsGroup {
		groupID = r.CreatorID
	}
	currentPlaceID := r.PlaceID

	filter := assetutils.NewFilter(ctx, r, assetTypeID)

	creatorPlaceMap := shardedmap.New[*atomicarray.AtomicArray[int64]]()
	creatorMutexMap := shardedmap.New[*sync.RWMutex]()

	// rate limit comes from the tuning package so the desktop app can scale it
	// with the user's pc specs at launch (default is 120/min matching the
	// previous hardcoded value)
	uploadQueue := taskqueue.New[int64](time.Minute, tuning.SoundUploadsPerMinute())
	permissionQueue := taskqueue.New[*assets.PermissionResponse](time.Minute, 60)
	permissionRequest := assetutils.NewPermissionBodyFromIds([]int64{r.UniverseID})

	logger.Println("Reuploading sounds...")

	newBatchError := func(amt int, m string, err any) {
		end := int(idsProcessed.Add(int32(amt)))
		start := end - amt
		logger.Error(uploaderror.NewBatch(start, end, idsToUpload, m, err))
	}

	newUploadError := func(m string, assetInfo *develop.AssetInfo, err any) {
		newValue := idsProcessed.Add(1)
		logger.Error(uploaderror.New(int(newValue), idsToUpload, m, assetInfo, err))
		if assetInfo != nil {
			// see animationgo: surface this to the desktop app via sse so
			// the user sees per-id failures during the job not just at the end
			resp.AddFailure(assetInfo.ID, m)
		}
	}

	grantPermissions := func(newID int64) (*assets.PermissionResponse, error) {
		permissionsClient, _ := roblox.NewClient("")
		permissionsClient.Cookie = client.Cookie
		permissionsClient.SetToken(client.GetToken())

		permissionHandler, err := assets.NewUpdatePermissionsHandler(client, newID, permissionRequest)
		if err != nil {
			return nil, err
		}

		res := <-permissionQueue.QueueTask(func() (*assets.PermissionResponse, error) {
			return retry.Do(
				retry.NewOptions(retry.Tries(3)),
				func(try int) (*assets.PermissionResponse, error) {
					pauseController.WaitIfPaused()
					if try > 1 {
						uploadQueue.Limiter.Wait()
					}

					permissionReponse, err := permissionHandler()
					if err == nil {
						return permissionReponse, nil
					}

					if err == assets.UpdatePermissionErrors.ErrNotAuthenticated {
						return nil, &retry.ExitRetry{Err: err}
					} else {
						switch err.(type) {
						case *net.OpError, *net.DNSError:
							permissionQueue.Limiter.Decrement()
						}
					}

					return nil, &retry.ContinueRetry{Err: err}
				},
			)
		})
		return res.Result, res.Error
	}

	uploadAsset := func(wg *sync.WaitGroup, assetInfo *develop.AssetInfo, location string) {
		defer wg.Done()

		if cancelController.IsCancelled() {
			return
		}

		oldName := assetInfo.Name
		uploadStart := time.Now()

		assetData, err := clientutils.GetRequest(client, location)
		if err != nil {
			newUploadError("Failed to get asset data", assetInfo, err)
			return
		}

		// pool is non-nil when the user supplied multiple credentials when there's
		// just one cookie ctxpool is nil and we use the single client directly
		// either way on quota or auth error we rotate to the next pool client and
		// rebuild the upload handler with its credentials
		pool := ctx.Pool
		var uploadingClient *roblox.Client
		if pool != nil && pool.HasAny() {
			c, _, err := pool.Acquire()
			if err != nil {
				newUploadError("No available clients", assetInfo, err)
				return
			}
			uploadingClient = c
		} else {
			uploadingClient = client
		}

		uploadHandler, err := publish.NewUploadAudioHandler(uploadingClient, assetInfo.Name, assetData, groupID)
		if err != nil {
			newUploadError("Failed to get upload handler", assetInfo, err)
			return
		}

		res := <-uploadQueue.QueueTask(func() (int64, error) {
			return retry.Do(
				retry.NewOptions(retry.Tries(8)),
				func(try int) (int64, error) {
					pauseController.WaitIfPaused()
					if cancelController.IsCancelled() {
						return 0, &retry.ExitRetry{Err: errors.New("cancelled")}
					}
					if try > 1 {
						uploadQueue.Limiter.Wait()
					}

					uploadResponse, err := uploadHandler()
					if err == nil {
						return uploadResponse.ID, nil
					}

					switch err {
					case publish.UploadAudioErrors.ErrNotAuthenticated:
						// pool mode: mark this client invalid rotate and rebuild handler
						// solo mode: prompt for new cookie like before (which now self-cancels
						// when no terminal is attached see clientutilsgetnewcookie)
						if pool != nil && pool.HasAny() {
							pool.MarkInvalid(uploadingClient)
							next, _, perr := pool.Acquire()
							if perr != nil {
								// no clients left surface the reason and exit cleanly
								if cancelController != nil && !cancelController.IsCancelled() {
									cancelController.CancelWithReason(true, "all account cookies invalid or rejected")
									ctx.ForceUnblockPause()
								}
								return 0, &retry.ExitRetry{Err: err}
							}
							uploadingClient = next
							newHandler, herr := publish.NewUploadAudioHandler(uploadingClient, assetInfo.Name, assetData, groupID)
							if herr == nil {
								uploadHandler = newHandler
							}
						} else {
							clientutils.GetNewCookie(ctx, r, "cookie expired")
						}
					case publish.UploadAudioErrors.ErrQuotaExceeded:
						// pool mode: this account is done rotate to the next
						// solo mode: nothing to do cancel with reason like before
						if pool != nil && pool.HasAny() {
							pool.MarkExhausted(uploadingClient)
							next, _, perr := pool.Acquire()
							if perr != nil {
								// no clients left across the whole pool
								if cancelController != nil && !cancelController.IsCancelled() {
									cancelController.CancelWithReason(true, "all accounts hit their upload quota")
									ctx.ForceUnblockPause()
								}
								return 0, &retry.ExitRetry{Err: err}
							}
							uploadingClient = next
							newHandler, herr := publish.NewUploadAudioHandler(uploadingClient, assetInfo.Name, assetData, groupID)
							if herr == nil {
								uploadHandler = newHandler
							}
						} else {
							if cancelController != nil && !cancelController.IsCancelled() {
								cancelController.CancelWithReason(true, "audio upload quota exceeded, you've hit your roblox upload limit")
								ctx.ForceUnblockPause()
							}
							return 0, &retry.ExitRetry{Err: err}
						}
					case publish.UploadAudioErrors.ErrModerated:
						assetInfo.Name = fmt.Sprintf("(%s) [Censored]", assetInfo.Name)
					default:
						switch err.(type) {
						case *net.OpError, *net.DNSError:
							uploadQueue.Limiter.Decrement()
						}
					}

					return 0, &retry.ContinueRetry{Err: err}
				},
			)
		})
		if err := res.Error; err != nil {
			assetInfo.Name = oldName
			newUploadError("Failed to upload", assetInfo, err)
			return
		}

		newID := res.Result
		elapsedMs := time.Since(uploadStart).Milliseconds()

		// record speed against the pool slot so the status endpoint can report avg ms/upload
		if pool != nil {
			pool.RecordUpload(uploadingClient, elapsedMs)
		}

		// after a successful upload grant every configured holder use permission
		// on the new asset id in pool mode the holders may differ from the
		// uploader in single-account mode this runs when permissionsharing is on
		// fire-and-forget: failures are logged but don't fail the upload
		var holderIDs []string
		if pool != nil {
			holderIDs = pool.HolderIDs()
		}
		if len(holderIDs) == 0 && r.PermissionSharing {
			for _, id := range r.HolderUserIDs {
				if id > 0 {
					holderIDs = append(holderIDs, fmt.Sprintf("%d", id))
				}
			}
		}
		if len(holderIDs) > 0 {
			go grantHolderPermission(uploadingClient, newID, holderIDs, ctx.Logger.Error)
		}

		newValue := idsProcessed.Add(1)
		logger.Success(uploaderror.New(int(newValue), idsToUpload, "", assetInfo, newID))
		resp.AddItem(response.ResponseItem{
			OldID: assetInfo.ID,
			NewID: newID,
		})

		if _, err = grantPermissions(newID); err != nil {
			message := fmt.Sprintf(">> %s(%d) failed to grant permission: ", assetInfo.Name, newID) + err.Error()
			if pauseController.IsPaused {
				color.Error.Fprintln(logger.History, message)
			} else {
				logger.Error(message)
			}
		} else {
			message := fmt.Sprintf(">> %s(%d) granted permission", assetInfo.Name, newID)
			if pauseController.IsPaused {
				color.Info.Fprintln(logger.History, message)
			} else {
				logger.Info(message)
			}
		}
	}

	getCreatorPlaceCache := func(creatorID int64, creatorType string) (*atomicarray.AtomicArray[int64], error) {
		creatorShard, exists := creatorPlaceMap.GetShard(creatorType)
		mutexShard, _ := creatorMutexMap.GetShard(creatorType)
		if !exists {
			creatorShard = creatorPlaceMap.NewShard(creatorType)
			mutexShard = creatorMutexMap.NewShard(creatorType)
		}

		if cache, cacheExists := creatorShard.Get(creatorID); cacheExists {
			return cache, nil
		}

		mutex, mutexExists := mutexShard.Get(creatorID)
		if !mutexExists {
			mutex = &sync.RWMutex{}
			mutexShard.Set(creatorID, mutex)
		}

		mutex.Lock()
		defer mutex.Unlock()

		if cache, cacheExists := creatorShard.Get(creatorID); cacheExists {
			return cache, nil
		}

		var resp *games.GamesResponse
		var err error
		if creatorType == "Group" {
			resp, err = games.GroupGames(client, creatorID)
		} else {
			resp, err = games.UserGames(client, creatorID)
		}
		if err != nil {
			return nil, err
		}

		ids := make([]int64, 0, len(defaultPlaceIDs)+len(resp.Data))
		ids = append(ids, defaultPlaceIDs...)
		for _, placeInfo := range resp.Data {
			rootPlaceID := placeInfo.RootPlace.ID

			if _, exists := defaultPlaceIDsMap[rootPlaceID]; exists {
				continue
			}

			ids = append(ids, rootPlaceID)
		}

		cache := atomicarray.New(&ids)
		creatorShard.Set(creatorID, cache)
		mutexShard.Remove(creatorID)
		return cache, nil
	}

	getAssetLocations := func(body []*assetdelivery.AssetRequestItem, placeID int64) ([]*assetdelivery.AssetLocation, error) {
		handler, err := assetdelivery.NewBatchHandler(client, body, placeID)
		if err != nil {
			return nil, err
		}

		return retry.Do(
			retry.NewOptions(retry.Tries(3)),
			func(try int) ([]*assetdelivery.AssetLocation, error) {
				pauseController.WaitIfPaused()

				locations, err := handler()
				if err != nil {
					return locations, &retry.ContinueRetry{Err: err}
				}

				for _, assetLocation := range locations {
					errs := assetLocation.Errors
					if errs == nil {
						continue
					}
					if errs[0].Message == "Authentication required to access Asset." {
						clientutils.GetNewCookie(ctx, r, "cookie expired")
						return locations, &retry.ContinueRetry{Err: ErrUnauthorized}
					}
				}

				return locations, nil
			},
		)
	}

	batchUpload := func(wg *sync.WaitGroup, creatorID int64, creatorType string, creatorAssets []*develop.AssetInfo) {
		defer wg.Done()

		placeCache, err := getCreatorPlaceCache(creatorID, creatorType)
		if err != nil {
			newBatchError(len(creatorAssets), "Failed to get creator places", err)
		}

		assetInfoMap := make(map[int64]*develop.AssetInfo)
		ids := make([]int64, len(creatorAssets))
		for i, assetInfo := range creatorAssets {
			ids[i] = assetInfo.ID
			assetInfoMap[assetInfo.ID] = assetInfo
		}
		body := assetutils.NewBatchBodyFromIDs(ids)

		var uploadWG sync.WaitGroup
		var assetLocations []*assetdelivery.AssetLocation
		for _, placeID := range placeCache.Load() {
			assetLocations, err = getAssetLocations(body, placeID)
			if err != nil {
				newBatchError(len(body), "Failed to get asset locations", err)
				return
			}

			var hadSuccess bool
			for assetIndex, assetLocation := range slices.Backward(assetLocations) {
				if len(assetLocation.Locations) == 0 {
					continue
				}
				hadSuccess = true

				assetID := body[assetIndex].AssetID
				body = slices.Delete(body, assetIndex, assetIndex+1)

				uploadWG.Add(1)
				go uploadAsset(&uploadWG, assetInfoMap[assetID], assetLocation.Locations[0].Location)
			}
			if hadSuccess {
				MoveValueToTop(placeCache, placeID)
			}
			if len(body) == 0 {
				break
			}
		}

		var index int
		for _, assetLocation := range assetLocations {
			if len(assetLocation.Locations) != 0 {
				continue
			}
			assetID := body[index].AssetID
			index++

			assetInfo := assetInfoMap[assetID]
			newUploadError("Failed to get asset location", assetInfo, assetLocation.Errors[0].Message)
		}

		uploadWG.Wait()
	}

	batchProcess := func(wg *sync.WaitGroup, res assetutils.AssetsInfoResult, batchSize int) {
		defer wg.Done()
		// abort whole batch early if user cancelled
		// the parent wgwait will see fewer add calls but the deferred done balances it
		if cancelController.IsCancelled() {
			return
		}
		assetsInfo := res.Result

		if err := res.Error; err != nil {
			newBatchError(batchSize, "Failed to get assets info", err)
			return
		}

		filteredInfo := filter(assetsInfo)
		filteredInfoLength := len(filteredInfo)
		// items removed by filter() are wrong type or already moderated/missing
		// these are reported as "wrong_type" since the studio plugin shouldn't
		// have sent them in the first place
		if skipped := batchSize - filteredInfoLength; skipped > 0 {
			resp.AddSkipped(response.SkipWrongType, skipped)
		}
		idsProcessed.Add(int32(batchSize - filteredInfoLength))
		if len(filteredInfo) == 0 {
			return
		}

		ids := make([]int64, filteredInfoLength)
		for i, assetInfo := range filteredInfo {
			ids[i] = assetInfo.ID
		}
		body := assetutils.NewBatchBodyFromIDs(ids)

		assetLocations, err := getAssetLocations(body, currentPlaceID)
		if err != nil {
			newBatchError(filteredInfoLength, "Failed to get asset locations to see permissions", err)
		}

		unownedAssets := make([]*develop.AssetInfo, 0)
		alreadyAccessible := 0
		for i, location := range assetLocations {
			if len(location.Locations) > 0 {
				// asset already has use permission for this place id so the existing
				// reference works and we don't need to reupload count as a skip with
				// the "already_accessible" reason for the desktop app's end-of-job summary
				alreadyAccessible++
				continue
			}

			unownedAssets = append(unownedAssets, filteredInfo[i])
		}
		if alreadyAccessible > 0 {
			resp.AddSkipped(response.SkipAlreadyAccessible, alreadyAccessible)
			// these skipped items were already counted in idsprocessed via the
			// filteredinfolength path above so we don't double-count here but
			// since we filtered them out of unownedassets the upload loop below
			// won't process them either need to bump idsprocessed manually since
			// the upload loop's per-item add calls won't fire for skipped items
			idsProcessed.Add(int32(alreadyAccessible))
		}

		CreatorAssets := make(map[string]map[int64][]*develop.AssetInfo)
		for _, assetInfo := range unownedAssets {
			assetCreatorType := assetInfo.Creator.Type
			assetCreatorID := assetInfo.Creator.TargetID

			creatorType, exists := CreatorAssets[assetCreatorType]
			if !exists {
				creatorType = make(map[int64][]*develop.AssetInfo)
				CreatorAssets[assetCreatorType] = creatorType
			}

			creatorAssets, exists := creatorType[assetCreatorID]
			if !exists {
				creatorAssets = make([]*develop.AssetInfo, 0)
				creatorType[assetCreatorID] = creatorAssets
			}

			creatorType[assetCreatorID] = append(creatorAssets, assetInfo)
		}

		var uploadWG sync.WaitGroup
		for creatorType, creatorAssetMap := range CreatorAssets {
			uploadWG.Add(len(creatorAssetMap))

			for creatorID, creatorAssets := range creatorAssetMap {
				go batchUpload(&uploadWG, creatorID, creatorType, creatorAssets)
			}
		}
		uploadWG.Wait()
	}

	var wg sync.WaitGroup
	tasks := assetutils.GetAssetsInfoInChunks(ctx, r)
	wg.Add(len(tasks))

	// cap concurrent batch goroutines to avoid spawning thousands at once
	// with large batches (500+ ids) the unbounded goroutine fan-out exhausts
	// memory and causes the pipeline to crash mid-job
	sem := make(chan struct{}, 8)

	for i, task := range tasks {
		batchSize := 50
		if i == len(tasks)-1 {
			batchSize = idsToUpload % 50
			if batchSize == 0 {
				batchSize = 50
			}
		}

		result := <-task
		sem <- struct{}{}
		go func(res assetutils.AssetsInfoResult, sz int) {
			defer func() { <-sem }()
			batchProcess(&wg, res, sz)
		}(result, batchSize)
	}
	wg.Wait()
}
