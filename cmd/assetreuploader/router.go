package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Obey72/backend/internal/app/assets"
	"github.com/Obey72/backend/internal/app/assets/shared/assetdb"
	"github.com/Obey72/backend/internal/app/config"
	appcontext "github.com/Obey72/backend/internal/app/context"
	"github.com/Obey72/backend/internal/app/request"
	"github.com/Obey72/backend/internal/app/response"
	"github.com/Obey72/backend/internal/app/tuning"
	"github.com/Obey72/backend/internal/color"
	"github.com/Obey72/backend/internal/files"
	"github.com/Obey72/backend/internal/roblox"
	"github.com/Obey72/backend/internal/session"
	"github.com/Obey72/backend/internal/ws"
)

var CompatiblePluginVersion = ""

type jobState struct {
	mu          sync.RWMutex
	busy        bool
	finished    bool
	lastError   string
	startedAt   time.Time
	totalIDs    int
	assetType   string
	cancelFn    func(keep bool) bool
	keepResults bool
	cancelled   bool
}

func (s *jobState) snapshot() (busy, finished bool, lastError, assetType string, total int, runMs int64) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var elapsed int64
	if !s.startedAt.IsZero() {
		elapsed = time.Since(s.startedAt).Milliseconds()
	}
	return s.busy, s.finished, s.lastError, s.assetType, s.totalIDs, elapsed
}

func (s *jobState) start(assetType string, total int) {
	s.mu.Lock()
	s.busy = true
	s.finished = false
	s.lastError = ""
	s.startedAt = time.Now()
	s.totalIDs = total
	s.assetType = assetType
	s.mu.Unlock()
}

func (s *jobState) end(err error) {
	s.mu.Lock()
	s.busy = false
	if err != nil {
		s.lastError = err.Error()
	}
	s.mu.Unlock()
}

func (s *jobState) markFinished() {
	s.mu.Lock()
	s.finished = true
	s.startedAt = time.Time{}
	s.totalIDs = 0
	s.assetType = ""
	s.mu.Unlock()
}

func (s *jobState) setError(msg string) {
	s.mu.Lock()
	s.lastError = msg
	s.mu.Unlock()
}

func (s *jobState) cancel(keep bool) bool {
	s.mu.Lock()
	cancelFn := s.cancelFn
	if !s.busy || s.cancelled || cancelFn == nil {
		s.mu.Unlock()
		return false
	}
	s.cancelled = true
	s.keepResults = keep
	s.mu.Unlock()
	return cancelFn(keep)
}

func (s *jobState) attachCancel(fn func(keep bool) bool) {
	s.mu.Lock()
	s.cancelFn = fn
	s.cancelled = false
	s.keepResults = false
	s.mu.Unlock()
}

func (s *jobState) wasCancelledKeep() (bool, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cancelled, s.keepResults
}

// analytics tracks per-job upload rates and counts
type analytics struct {
	mu          sync.Mutex
	uploaded    atomic.Int64
	failed      atomic.Int64
	jobStart    time.Time
	uploadTimes []int64 // ms per upload for avg speed
}

func (a *analytics) reset(start time.Time) {
	a.mu.Lock()
	a.uploaded.Store(0)
	a.failed.Store(0)
	a.jobStart = start
	a.uploadTimes = a.uploadTimes[:0]
	a.mu.Unlock()
}

func (a *analytics) recordUpload(ms int64) {
	a.uploaded.Add(1)
	a.mu.Lock()
	if len(a.uploadTimes) < 500 {
		a.uploadTimes = append(a.uploadTimes, ms)
	}
	a.mu.Unlock()
}

func (a *analytics) recordFailure() {
	a.failed.Add(1)
}

func (a *analytics) snapshot(totalIDs int) map[string]any {
	a.mu.Lock()
	uploaded := a.uploaded.Load()
	failed := a.failed.Load()
	times := make([]int64, len(a.uploadTimes))
	copy(times, a.uploadTimes)
	start := a.jobStart
	a.mu.Unlock()

	var avgms float64
	if len(times) > 0 {
		var sum int64
		for _, t := range times {
			sum += t
		}
		avgms = float64(sum) / float64(len(times))
	}

	elapsed := time.Since(start).Seconds()
	var permin float64
	if elapsed > 0 {
		permin = float64(uploaded) / elapsed * 60
	}

	remaining := int64(totalIDs) - uploaded - failed
	var etaSec float64
	if permin > 0 && remaining > 0 {
		etaSec = float64(remaining) / permin * 60
	}

	return map[string]any{
		"uploaded":   uploaded,
		"failed":     failed,
		"perMinute":  permin,
		"avgUploadMs": avgms,
		"etaSec":     etaSec,
	}
}

func getOutputFileName(reuploadType string) string {
	t := time.Now()
	return fmt.Sprintf("Output_%s_%s.json", reuploadType, t.Format("2006-01-02_15-04-05"))
}

func getDataDir() string {
	// backend runs with cwd = data folder (set by electron)
	if cwd, err := os.Getwd(); err == nil {
		return cwd
	}
	return "."
}

func cookiesuffix(cookie string) string {
	if len(cookie) <= 8 {
		return cookie
	}
	return cookie[len(cookie)-8:]
}

func serve(c *roblox.Client) error {
	var exportedJSONName string
	var exportJSON bool
	state := &jobState{finished: true}
	an := &analytics{}
	hub := ws.NewHub()

	datadir := getDataDir()
	session.Init(datadir)
	assetdb.Init(datadir)

	pool := roblox.NewPool()

	// settings the desktop app toggles independent of the plugin payload
	// both default off so existing flows are unchanged we mirror them into
	// the rawrequest in /reupload so the single-account path can use them
	var settingsMu sync.RWMutex
	var reuploadAlreadyOwned bool
	var permissionSharing bool

	respHistory := make([]response.ResponseItem, 0)
	resp := response.New(func(i response.ResponseItem) {
		if exportJSON {
			respHistory = append(respHistory, i)
			j, err := json.Marshal(respHistory)
			if err != nil {
				log.Fatal(err)
			}
			if err := files.Write(exportedJSONName, string(j)); err != nil {
				log.Fatal(err)
			}
		}
		// push live update to sse clients
		hub.Send("asset", map[string]any{
			"oldId": i.OldID,
			"newId": i.NewID,
		})
	})
	// fan out per-batch skip events so the desktop app can log "skip n (reason)"
	// lines mid-job instead of only seeing a totals breakdown at the end
	resp.SetOnSkipped(func(reason response.SkipReason, n int) {
		hub.Send("skip", map[string]any{
			"reason": string(reason),
			"count":  n,
		})
	})
	resp.SetOnFailed(func(oldID int64, reason string) {
		hub.Send("fail", map[string]any{
			"oldId":  oldID,
			"reason": reason,
		})
	})

	// get /events  sse stream for live updates (replaces renderer polling)
	http.HandleFunc("GET /events", hub.Handler)

	// wire the animreup endpoints (see animreup.go) the hub holds the task
	// queue + result store so app and plugin can rendezvous without sse the
	// roblox client is needed by the rbxdl-backed paths so single-id reuploads
	// can hit publish.roblox.com / ide.roblox.com without going through the plugin
	animHub := newAnimreupHub()
	setupAnimreupRoutes(animHub, c)

	// get /session  check if a restorable session exists
	http.HandleFunc("GET /session", func(w http.ResponseWriter, r *http.Request) {
		s, err := session.Load()
		w.Header().Set("Content-Type", "application/json")
		if err != nil || s == nil {
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, `{"exists":false}`)
			return
		}
		pending := len(s.PendingIDs)
		uploaded := len(s.Uploaded)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"exists":    true,
			"id":        s.ID,
			"assetType": s.AssetType,
			"pending":   pending,
			"uploaded":  uploaded,
			"startedAt": s.StartedAt,
		})
	})

	// delete /session  discard a saved session
	http.HandleFunc("DELETE /session", func(w http.ResponseWriter, r *http.Request) {
		session.Delete()
		w.WriteHeader(http.StatusOK)
	})

	// get /history  recent upload history from local db
	http.HandleFunc("GET /history", func(w http.ResponseWriter, r *http.Request) {
		n, _ := strconv.Atoi(r.URL.Query().Get("n"))
		if n <= 0 {
			n = 100
		}
		entries := assetdb.Recent(n)
		total, ok, fail := assetdb.Stats()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"entries": entries,
			"total":   total,
			"ok":      ok,
			"failed":  fail,
		})
	})

	http.HandleFunc("POST /set-credentials", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Cookie      string `json:"cookie"`
			APIKey      string `json:"apiKey"`
			Credentials []struct {
				Cookie string `json:"cookie"`
				APIKey string `json:"apiKey"`
			} `json:"credentials"`
			HolderID             string   `json:"holderId"`
			HolderIDs            []string `json:"holderIds"`
			ReuploadAlreadyOwned *bool    `json:"reuploadAlreadyOwned"`
			PermissionSharing    *bool    `json:"permissionSharing"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if body.Cookie != "" {
			if err := c.SetCookie(strings.TrimSpace(body.Cookie)); err != nil {
				color.Error.Println("set-credentials: cookie error:", err)
				state.setError("cookie auth failed: " + err.Error())
			} else {
				files.Write(cookieFile, strings.TrimSpace(body.Cookie)+"\n")
				fmt.Println("Cookie updated. user id:", c.UserInfo.ID)
			}
		}
		if body.APIKey != "" {
			config.Set("api_key", strings.TrimSpace(body.APIKey))
			config.PersistAPIKey()
			fmt.Println("API key updated.")
		}

		if len(body.Credentials) > 0 {
			newClients := make([]*roblox.Client, 0, len(body.Credentials))
			newKeys := make([]string, 0, len(body.Credentials))
			for i, cred := range body.Credentials {
				cookie := strings.TrimSpace(cred.Cookie)
				if cookie == "" {
					continue
				}
				var newC *roblox.Client
				var err error
				if existing := pool.FindByCookie(cookie); existing != nil && existing.UserInfo.ID != 0 {
					newC = existing
				} else {
					newC, err = roblox.NewClient(cookie)
					if err != nil {
						color.Error.Println(fmt.Sprintf("credential %d: cookie auth failed: %v", i, err))
					}
				}
				newClients = append(newClients, newC)
				newKeys = append(newKeys, strings.TrimSpace(cred.APIKey))
				if i == 0 && err == nil {
					if newC != nil && newC.Cookie != "" {
						c.Cookie = newC.Cookie
						c.UserInfo = newC.UserInfo
					}
					if cred.APIKey != "" {
						config.Set("api_key", strings.TrimSpace(cred.APIKey))
						config.PersistAPIKey()
					}
				}
			}
			pool.Replace(newClients, newKeys)
			// merge legacy single-holder with the new list so old clients keep
			// working while new ones can send multiple holders at once
			holders := make([]string, 0, len(body.HolderIDs)+1)
			if v := strings.TrimSpace(body.HolderID); v != "" {
				holders = append(holders, v)
			}
			for _, h := range body.HolderIDs {
				holders = append(holders, strings.TrimSpace(h))
			}
			pool.SetHolderIDs(holders)
			stats := pool.Stats()
			fmt.Printf("Pool updated: total=%d active=%d invalid=%d holders=%v\n",
				stats.Total, stats.Active, stats.Invalid, pool.HolderIDs())
		} else if len(body.HolderIDs) > 0 || strings.TrimSpace(body.HolderID) != "" {
			// permission sharing path no credentials list sent we still want
			// to track the holders so single-account jobs can grant on them
			holders := make([]string, 0, len(body.HolderIDs)+1)
			if v := strings.TrimSpace(body.HolderID); v != "" {
				holders = append(holders, v)
			}
			for _, h := range body.HolderIDs {
				holders = append(holders, strings.TrimSpace(h))
			}
			pool.SetHolderIDs(holders)
			fmt.Printf("Holders updated for permission sharing: %v\n", pool.HolderIDs())
		}

		// apply optional behaviour toggles from the desktop app pointers let
		// the renderer leave them untouched on calls that only update creds
		if body.ReuploadAlreadyOwned != nil || body.PermissionSharing != nil {
			settingsMu.Lock()
			if body.ReuploadAlreadyOwned != nil {
				reuploadAlreadyOwned = *body.ReuploadAlreadyOwned
			}
			if body.PermissionSharing != nil {
				permissionSharing = *body.PermissionSharing
			}
			settingsMu.Unlock()
		}

		w.WriteHeader(http.StatusOK)
	})

	http.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		busy, finished, _, _, _, _ := state.snapshot()
		if resp.Len() == 0 && !busy {
			if !finished {
				state.markFinished()
				exportJSON = false
				resp.Clear()
				respHistory = make([]response.ResponseItem, 0)
				fmt.Fprint(w, "done")
				fmt.Println("Finished reuploading. (you can rerun without restarting)")
				session.Delete()
			}
			return
		}
		if err := resp.EncodeJSON(json.NewEncoder(w)); err != nil {
			log.Fatal(err)
		} else {
			resp.Clear()
		}
	})

	http.HandleFunc("GET /status", func(w http.ResponseWriter, r *http.Request) {
		since, _ := strconv.Atoi(r.URL.Query().Get("since"))
		items := resp.HistorySince(since)
		total := resp.HistoryLen()
		busy, finished, lastError, assetType, totalIDs, runMs := state.snapshot()
		poolStats := pool.Stats()
		skipCounts := resp.SkippedCounts()
		skipTotal := resp.SkippedTotal()

		settingsMu.RLock()
		curReuploadOwned := reuploadAlreadyOwned
		curPermissionSharing := permissionSharing
		settingsMu.RUnlock()

		out := map[string]any{
			"busy":           busy,
			"finished":       finished,
			"totalProcessed": total,
			"items":          items,
			"lastError":      lastError,
			"assetType":      assetType,
			"totalIds":       totalIDs,
			"runMs":          runMs,
			"cookieOk":       c.UserInfo.ID != 0,
			"userId":         c.UserInfo.ID,
			"skipped":        skipCounts,
			"skippedTotal":   skipTotal,
			"pool": map[string]any{
				"total":     poolStats.Total,
				"active":    poolStats.Active,
				"exhausted": poolStats.Exhausted,
				"invalid":   poolStats.Invalid,
				"holderId":  pool.HolderID(),
				"holderIds": pool.HolderIDs(),
				"slots":     poolStats.Slots,
			},
			"reuploadAlreadyOwned": curReuploadOwned,
			"permissionSharing":    curPermissionSharing,
		}
		if busy {
			out["analytics"] = an.snapshot(totalIDs)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(out)
	})

	http.HandleFunc("POST /reupload", func(w http.ResponseWriter, r *http.Request) {
		busy, finished, _, _, _, _ := state.snapshot()
		if busy || !finished {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		var req request.RawRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			color.Error.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// merge the desktop-app side settings into the incoming plugin
		// request the plugin doesn't know about these toggles
		settingsMu.RLock()
		if reuploadAlreadyOwned {
			req.ReuploadAlreadyOwned = true
		}
		if permissionSharing {
			req.PermissionSharing = true
		}
		settingsMu.RUnlock()

		// pull holders from the pool into the request so the single-account
		// path can grant permission without the plugin needing to send them
		if storedHolders := pool.HolderIDs(); len(storedHolders) > 0 {
			seen := make(map[int64]struct{})
			for _, id := range req.HolderUserIDs {
				if id > 0 {
					seen[id] = struct{}{}
				}
			}
			if req.HolderUserID > 0 {
				seen[req.HolderUserID] = struct{}{}
			}
			for _, h := range storedHolders {
				n, err := strconv.ParseInt(h, 10, 64)
				if err != nil || n <= 0 {
					continue
				}
				if _, ok := seen[n]; ok {
					continue
				}
				seen[n] = struct{}{}
				req.HolderUserIDs = append(req.HolderUserIDs, n)
			}
		}
		if CompatiblePluginVersion != "" && req.PluginVersion != CompatiblePluginVersion {
			w.WriteHeader(http.StatusConflict)
			return
		}
		if exists := assets.DoesModuleExist(req.AssetType); !exists {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if c.UserInfo.ID == 0 {
			msg := "no valid cookie set, paste a fresh .ROBLOSECURITY cookie in the desktop app"
			state.setError(msg)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
			return
		}

		resp.ClearHistory()
		state.start(req.AssetType, len(req.IDs))
		an.reset(time.Now())

		// save session so we can restore if the app crashes mid-job
		sess := &session.Session{
			ID:        fmt.Sprintf("%d", time.Now().UnixMilli()),
			AssetType: req.AssetType,
			PendingIDs: req.IDs,
			StartedAt: time.Now().UnixMilli(),
			PlaceID:   req.PlaceID,
		}
		session.Save(sess)

		var ctx *appcontext.Context
		startReupload, err := assets.NewReuploadHandlerWithPool(req.AssetType, c, pool, &req, resp, func(uctx *appcontext.Context) {
			ctx = uctx
		})
		if err != nil {
			color.Error.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			session.Delete()
			return
		}
		if exportJSON = req.ExportJSON; exportJSON {
			exportedJSONName = getOutputFileName(req.AssetType)
		}

		state.attachCancel(func(keep bool) bool {
			if ctx == nil || ctx.CancelController == nil {
				return false
			}
			ctx.CancelController.Cancel(keep)
			ctx.ForceUnblockPause()
			return true
		})

		// broadcast job start to sse clients
		hub.Send("start", map[string]any{
			"assetType": req.AssetType,
			"totalIds":  len(req.IDs),
		})

		go func() {
			start := time.Now()
			err := startReupload()
			cancelled, keep := state.wasCancelledKeep()
			if err != nil {
				state.end(fmt.Errorf("upload pipeline failed: %w", err))
				color.Error.Println("Failed to start reuploading: ", err)
				if errors.Is(err, roblox.ErrEmptyCookie) {
					state.setError("no cookie set")
				}
				session.Delete()
				hub.Send("done", map[string]any{"error": err.Error()})
				return
			}
			if cancelled && !keep {
				resp.Clear()
				resp.ClearHistory()
				reason := "cancelled, results discarded"
				if ctx != nil && ctx.CancelController != nil {
					if r := ctx.CancelController.Reason(); r != "" {
						reason = r
					}
				}
				state.setError(reason)
			} else if cancelled {
				reason := "cancelled, replacing partial results"
				if ctx != nil && ctx.CancelController != nil {
					if r := ctx.CancelController.Reason(); r != "" {
						reason = r
					}
				}
				state.setError(reason)
			}
			state.end(nil)
			session.Delete()
			duration := time.Since(start)
			fmt.Printf("Reuploading took %d hours, %d minutes, and %d seconds\n", int(duration.Hours()), int(duration.Minutes())%60, int(duration.Seconds())%60)
			fmt.Println("Waiting for client to finish changing ids...")
			hub.Send("done", map[string]any{
				"elapsed": duration.Milliseconds(),
				"analytics": an.snapshot(state.totalIDs),
			})
		}()
		w.WriteHeader(http.StatusOK)
	})

	http.HandleFunc("POST /cancel", func(w http.ResponseWriter, r *http.Request) {
		keep := r.URL.Query().Get("keep") != "false"
		if state.cancel(keep) {
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "cancel issued, keep=%v", keep)
			return
		}
		w.WriteHeader(http.StatusConflict)
		fmt.Fprint(w, "no job to cancel")
	})

	// post /analytics/upload  called by the sound pipeline after each upload
	// records timing data without the pipeline needing a direct reference to analytics
	http.HandleFunc("POST /analytics/upload", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Ms      int64  `json:"ms"`
			OldID   int64  `json:"oldId"`
			NewID   int64  `json:"newId"`
			Cookie  string `json:"cookie"`
			AssetType string `json:"assetType"`
			Failed  bool   `json:"failed"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if body.Failed {
			an.recordFailure()
			assetdb.RecordFailure(body.OldID, body.AssetType, "upload failed", cookiesuffix(body.Cookie))
			// broadcast so the desktop app's log shows "no <oldid> (upload failed)"
			// inline with the ok lines instead of only learning about failures at the end
			hub.Send("fail", map[string]any{
				"oldId":  body.OldID,
				"reason": "upload failed",
			})
		} else {
			an.recordUpload(body.Ms)
			assetdb.RecordSuccess(body.OldID, body.NewID, body.AssetType, cookiesuffix(body.Cookie))
			pool.RecordUpload(nil, body.Ms)
		}
		w.WriteHeader(http.StatusOK)
	})

	http.HandleFunc("POST /theme-assist", themeAssistHandler)

	// post /tuning  desktop app calls this at launch after detecting cpu/ram
	// so we can scale concurrency and request rates instead of using a single
	// hardcoded value any field <= 0 is ignored so the app can update one
	// knob without touching the others takes effect on the *next* job start
	// (in-flight jobs keep their queue settings)
	http.HandleFunc("POST /tuning", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			AnimationStartsPerMinute int `json:"animationStartsPerMinute"`
			AnimationMaxConcurrent   int `json:"animationMaxConcurrent"`
			SoundUploadsPerMinute    int `json:"soundUploadsPerMinute"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		tuning.Apply(body.AnimationStartsPerMinute, body.AnimationMaxConcurrent, body.SoundUploadsPerMinute)
		fmt.Printf("tuning updated: %+v\n", tuning.Snapshot())
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(tuning.Snapshot())
	})

	// get /tuning  read back the current tuning values lets the desktop app
	// confirm what the backend is actually running with after apply
	http.HandleFunc("GET /tuning", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(tuning.Snapshot())
	})

	// get /yt-searchq=  backend-side youtube search via invidious the renderer
	// fetches this when webview csp / browser-side network blocks direct invidious calls
	// returns a normalized array of { videoid title author lengthseconds viewcount publishedtext }
	http.HandleFunc("GET /yt-search", func(w http.ResponseWriter, r *http.Request) {
		q := strings.TrimSpace(r.URL.Query().Get("q"))
		w.Header().Set("Content-Type", "application/json")
		if q == "" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("[]"))
			return
		}

		mirrors := []string{
			"https://invidious.privacyredirect.com",
			"https://iv.datura.network",
			"https://invidious.nerdvpn.de",
			"https://invidious.incogniweb.net",
			"https://yewtu.be",
			"https://inv.tux.pizza",
			"https://invidious.flokinet.to",
			"https://invidious.materialio.us",
		}
		rand.Shuffle(len(mirrors), func(i, j int) { mirrors[i], mirrors[j] = mirrors[j], mirrors[i] })

		client := &http.Client{Timeout: 6 * time.Second}
		fields := "videoId,title,author,lengthSeconds,viewCount,publishedText"
		path := "/api/v1/search?type=video&fields=" + fields + "&q=" + url.QueryEscape(q)

		for _, host := range mirrors {
			req, err := http.NewRequest("GET", host+path, nil)
			if err != nil {
				continue
			}
			req.Header.Set("User-Agent", "Mozilla/5.0 (reup-search)")
			resp, err := client.Do(req)
			if err != nil {
				continue
			}
			body, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil || resp.StatusCode != http.StatusOK {
				continue
			}
			var arr []map[string]any
			if json.Unmarshal(body, &arr) != nil || len(arr) == 0 {
				continue
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(body)
			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("[]"))
	})

	// place-name lookup queue: when the app types a placeid that the public roblox api can't
	// resolve (private place / cookie-locked) the app posts /lookup-place the plugin polls
	// /pending-place-lookups calls marketplaceservice:getproductinfo (which uses the studio
	// user's session and can see private places they have access to) then posts the name back
	// through /plugin-data the sse broadcast pushes it to the app
	var lookupMu sync.Mutex
	pendingLookups := make(map[string]struct{}) // placeid set drained by plugin poll
	resolvedNames := make(map[string]bool)      // placeids we already have a name for skip queueing

	http.HandleFunc("POST /lookup-place", func(w http.ResponseWriter, r *http.Request) {
		var body struct{ PlaceID string `json:"placeId"` }
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.PlaceID == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		lookupMu.Lock()
		if !resolvedNames[body.PlaceID] {
			pendingLookups[body.PlaceID] = struct{}{}
		}
		lookupMu.Unlock()
		w.WriteHeader(http.StatusOK)
	})

	http.HandleFunc("GET /pending-place-lookups", func(w http.ResponseWriter, r *http.Request) {
		lookupMu.Lock()
		ids := make([]string, 0, len(pendingLookups))
		for id := range pendingLookups {
			ids = append(ids, id)
		}
		// drain  plugin is expected to resolve and post back via /plugin-data
		pendingLookups = make(map[string]struct{})
		lookupMu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(ids)
	})

	// post /plugin-data  plugin sends its current placeid username and (when known) placename
	// the app keeps a running list keyed by placeid; placename is preserved across updates so a
	// later post without a name doesn't clobber a previously-known name
	var pluginDataMu sync.Mutex
	type pluginEntry struct {
		PlaceID   string `json:"placeId"`
		Username  string `json:"username"`
		PlaceName string `json:"placeName"`
	}
	pluginPlaceMap := make(map[string]pluginEntry) // placeid -> entry

	http.HandleFunc("POST /plugin-data", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			PlaceID   string `json:"placeId"`
			Username  string `json:"username"`
			PlaceName string `json:"placeName"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.PlaceID == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		pluginDataMu.Lock()
		existing := pluginPlaceMap[body.PlaceID]
		name := body.PlaceName
		if name == "" {
			name = existing.PlaceName
		}
		entry := pluginEntry{PlaceID: body.PlaceID, Username: body.Username, PlaceName: name}
		pluginPlaceMap[body.PlaceID] = entry
		pluginDataMu.Unlock()
		if name != "" {
			lookupMu.Lock()
			resolvedNames[body.PlaceID] = true
			delete(pendingLookups, body.PlaceID)
			lookupMu.Unlock()
		}
		fmt.Printf("plugin-data: placeId=%s username=%s placeName=%q\n", body.PlaceID, body.Username, name)
		hub.Send("plugin-place", map[string]any{
			"placeId":   body.PlaceID,
			"username":  body.Username,
			"placeName": name,
		})
		w.WriteHeader(http.StatusOK)
	})

	// get /plugin-places  returns all known {placeid username placename} entries the plugin has reported
	http.HandleFunc("GET /plugin-places", func(w http.ResponseWriter, r *http.Request) {
		pluginDataMu.Lock()
		entries := make([]pluginEntry, 0, len(pluginPlaceMap))
		for _, e := range pluginPlaceMap {
			entries = append(entries, e)
		}
		pluginDataMu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(entries)
	})

	// bind to loopback only  listening on 0000 triggers a windows firewall
	// prompt the first time the backend runs and a "cancel" click creates a
	// permanent block rule the user has to dig out of wfmsc loopback isn't
	// filtered by default firewall and both the desktop app and the studio
	// plugin only ever connect via 127001 so this is a strict downgrade
	// of attack surface with no functional cost
	return http.ListenAndServe("127.0.0.1:"+port, corsMiddleware(http.DefaultServeMux))
}
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}


func themeAssistHandler(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Prompt string `json:"prompt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Prompt == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	prompt := strings.ToLower(body.Prompt)

	isDark := !strings.Contains(prompt, "light") &&
		!strings.Contains(prompt, "white") &&
		!strings.Contains(prompt, "bright")

	accentMap := map[string][3]int{
		"red":     {220, 50, 50},
		"orange":  {255, 130, 30},
		"gold":    {230, 190, 50},
		"yellow":  {250, 220, 20},
		"green":   {60, 220, 80},
		"lime":    {110, 255, 60},
		"teal":    {0, 210, 200},
		"cyan":    {0, 210, 240},
		"blue":    {60, 140, 255},
		"cobalt":  {30, 80, 200},
		"purple":  {160, 80, 255},
		"violet":  {140, 60, 240},
		"pink":    {255, 100, 180},
		"magenta": {255, 40, 200},
		"white":   {240, 240, 240},
		"mint":    {122, 240, 212},
		"rose":    {220, 100, 140},
	}

	accent := [3]int{160, 80, 255}
	for kw, rgb := range accentMap {
		if strings.Contains(prompt, kw) {
			accent = rgb
			break
		}
	}

	var bgR, bgG, bgB, fgR, fgG, fgB int
	if isDark {
		bgR, bgG, bgB = 12, 8, 20
		fgR, fgG, fgB = 20, 14, 34
	} else {
		bgR, bgG, bgB = 248, 244, 252
		fgR, fgG, fgB = 236, 228, 248
		accent = [3]int{
			clamp(int(float64(accent[0])*0.7), 0, 255),
			clamp(int(float64(accent[1])*0.7), 0, 255),
			clamp(int(float64(accent[2])*0.7), 0, 255),
		}
	}

	font := "JetBrainsMono"
	switch {
	case strings.Contains(prompt, "serif") || strings.Contains(prompt, "elegant") || strings.Contains(prompt, "classic"):
		font = "Merriweather"
	case strings.Contains(prompt, "round") || strings.Contains(prompt, "soft") || strings.Contains(prompt, "cute"):
		font = "Nunito"
	case strings.Contains(prompt, "bold") || strings.Contains(prompt, "strong") || strings.Contains(prompt, "impact"):
		font = "Oswald"
	case strings.Contains(prompt, "clean") || strings.Contains(prompt, "minimal"):
		font = "Gotham"
	}

	borderR := clamp(bgR*2+30, 0, 255)
	borderG := clamp(bgG*2+30, 0, 255)
	borderB := clamp(bgB*2+30, 0, 255)
	unselR := clamp(bgR+bgR/2+10, 0, 255)
	unselG := clamp(bgG+bgG/2+10, 0, 255)
	unselB := clamp(bgB+bgB/2+10, 0, 255)

	var textR, textG, textB, dimR, dimG, dimB int
	if isDark {
		textR, textG, textB = 220, 215, 235
		dimR, dimG, dimB = 110, 100, 140
	} else {
		textR, textG, textB = 30, 20, 50
		dimR, dimG, dimB = 160, 145, 190
	}

	rawTheme := map[string]any{
		"Name":                  body.Prompt,
		"Font":                  font,
		"TextSize":              13,
		"BackgroundColor":       []int{bgR, bgG, bgB},
		"ForegroundColor":       []int{fgR, fgG, fgB},
		"BorderColor":           []int{borderR, borderG, borderB},
		"UnselectedColor":       []int{unselR, unselG, unselB},
		"SelectedColor":         []int{accent[0], accent[1], accent[2]},
		"TextColor":             []int{textR, textG, textB},
		"DimmedTextColor":       []int{dimR, dimG, dimB},
		"ToggleBackgroundColor": []int{accent[0], accent[1], accent[2]},
		"TipColor":              []int{accent[0], accent[1], accent[2]},
		"InputColor":            []int{textR, textG, textB},
		"StrokeTransparency":    1,
	}

	themeJSON, err := json.Marshal(rawTheme)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"reply": string(themeJSON),
	})
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
