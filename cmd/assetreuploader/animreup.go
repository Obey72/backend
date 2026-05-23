package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/Obey72/backend/internal/color"
	"github.com/Obey72/backend/internal/roblox"
	"github.com/Obey72/backend/internal/roblox/ide"
	"github.com/Obey72/backend/internal/roblox/publish"
)

// --- animreup task queue
// the animreup tab in the desktop app needs to reach into studio for things only
// the plugin can do (read a keyframesequence, run a reupload through the
// existing pipeline) we model this as a tiny task queue: the app POSTs a task,
// the plugin long-polls /animreup/next-task, executes it in studio, posts the
// result back the app polls /animreup/result/{taskId} until the result lands
// short-poll fallback: clients that can't long-poll (or hit the long-poll
// timeout) just keep calling /animreup/next-task and get a 204 when there's
// nothing to hand out
// note: this lives in package main next to router.go so it shares state with
// the existing http handler registrations no init function, call
// setupAnimreupRoutes(...) from serve() to wire it up

type animreupTask struct {
	ID         string    `json:"taskId"`        // server-issued opaque id
	Kind       string    `json:"kind"`          // "fetch_keyframes" | "reupload"
	AssetID    int64     `json:"assetId"`       // roblox asset id the task is about
	AssetType  string    `json:"assetType"`     // "animation" | "sound"
	CreatedAt  time.Time `json:"-"`             // for stale-task gc
	deliveredC chan struct{}                    // closed when a plugin picks up the task
}

type animreupResult struct {
	Done       bool        `json:"-"`
	Payload    interface{} `json:"-"` // free-form result blob from the plugin
	Error      string      `json:"-"`
	FinishedAt time.Time   `json:"-"`
}

type animreupHub struct {
	mu       sync.Mutex
	queue    []*animreupTask                // fifo of unclaimed tasks
	results  map[string]*animreupResult     // taskId → result (or in-progress placeholder)
	wakeups  []chan struct{}                // long-poll wakeup channels (one per waiter)
	lastPoll time.Time                      // wall-time of the most recent /animreup/next-task hit
}

func newAnimreupHub() *animreupHub {
	return &animreupHub{
		results: make(map[string]*animreupResult),
	}
}

// markpolled updates the last-poll timestamp the app's plugin-alive probe
// reads this to surface "plugin not detected" before the user waits out a
// 30s task timeout
func (h *animreupHub) markPolled() {
	h.mu.Lock()
	h.lastPoll = time.Now()
	h.mu.Unlock()
}

func (h *animreupHub) lastPollAge() time.Duration {
	h.mu.Lock()
	t := h.lastPoll
	h.mu.Unlock()
	if t.IsZero() {
		return time.Hour * 24 // arbitrary "never"
	}
	return time.Since(t)
}

// pushtask enqueues a new task and wakes any waiting long-polls returns the
// task id which the app uses to poll for the result
func (h *animreupHub) pushTask(kind, assetType string, assetID int64) *animreupTask {
	t := &animreupTask{
		ID:         fmt.Sprintf("t-%d-%d", time.Now().UnixNano(), assetID),
		Kind:       kind,
		AssetID:    assetID,
		AssetType:  assetType,
		CreatedAt:  time.Now(),
		deliveredC: make(chan struct{}),
	}
	h.mu.Lock()
	h.queue = append(h.queue, t)
	// stub the result so the app's first poll returns "pending" instead of 404
	h.results[t.ID] = &animreupResult{}
	// drain the wakeup channels so any sleeping long-poll wakes up and re-checks
	for _, w := range h.wakeups {
		select { case w <- struct{}{}: default: }
	}
	h.wakeups = h.wakeups[:0]
	h.mu.Unlock()
	fmt.Printf("animreup: queued task %s kind=%s id=%d\n", t.ID, t.Kind, t.AssetID)
	return t
}

// claimnext pops the oldest task from the queue if there is one
// returns nil when the queue is empty so callers can choose to wait or 204
func (h *animreupHub) claimNext() *animreupTask {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.queue) == 0 {
		return nil
	}
	t := h.queue[0]
	h.queue = h.queue[1:]
	close(t.deliveredC)
	return t
}

// waitfortask blocks until a task arrives or the timeout fires returns nil on
// timeout so the handler can write 204, that's how the short-poll fallback
// distinguishes "nothing yet" from "go ahead and process this"
func (h *animreupHub) waitForTask(timeout time.Duration) *animreupTask {
	// fast path
	if t := h.claimNext(); t != nil {
		return t
	}
	wake := make(chan struct{}, 1)
	h.mu.Lock()
	h.wakeups = append(h.wakeups, wake)
	h.mu.Unlock()
	select {
	case <-wake:
		return h.claimNext()
	case <-time.After(timeout):
		// drop our wakeup channel so we don't leak; tolerate the race where
		// pushtask is about to deliver to us anyway by retrying claimnext once
		h.mu.Lock()
		for i, w := range h.wakeups {
			if w == wake {
				h.wakeups = append(h.wakeups[:i], h.wakeups[i+1:]...)
				break
			}
		}
		h.mu.Unlock()
		return h.claimNext()
	}
}

func (h *animreupHub) storeResult(taskID string, payload interface{}, errMsg string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.results[taskID] = &animreupResult{
		Done:       true,
		Payload:    payload,
		Error:      errMsg,
		FinishedAt: time.Now(),
	}
}

func (h *animreupHub) getResult(taskID string) *animreupResult {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.results[taskID]
}

// gcResults reaps results older than 10 minutes so the map doesn't grow without
// bound on a long-running session run this from a goroutine on startup
func (h *animreupHub) gcResults() {
	for {
		time.Sleep(2 * time.Minute)
		cutoff := time.Now().Add(-10 * time.Minute)
		h.mu.Lock()
		for id, r := range h.results {
			if r.Done && r.FinishedAt.Before(cutoff) {
				delete(h.results, id)
			}
		}
		h.mu.Unlock()
	}
}

// --- http handlers
// setupanimreuproutes registers every /animreup/* handler on the default mux
// called from serve() in router.go pluginCtx supplies the most recent place /
// creator info the plugin has reported so the single-id reupload can hand
// the full request.rawrequest to the existing /reupload pipeline

// setupAnimreupRoutes registers every /animreup/* handler on the default mux.
// the plugin-driven paths (fetch/reupload via studio long-poll) still work for
// users who have studio open with the plugin attached. the new rbxdl-* paths
// route the single-id animreup overlay through https://rbxdl.johnmarctumulak.com
// instead, so the desktop app can preview + reupload one-off assets without
// requiring studio. the rbxdl-* paths need the roblox client (for the cookie
// that publish.roblox.com expects) so we accept it here.
func setupAnimreupRoutes(hub *animreupHub, c *roblox.Client) {
	rbxdl := newRbxdlClient()
	setupAnimreupRbxdlRoutes(rbxdl, c)

	go hub.gcResults()

	// POST /animreup/fetch , app queues a keyframe fetch task
	// body: { id: number, type: "animation" | "sound" }
	// returns: { taskId: "..." }
	http.HandleFunc("POST /animreup/fetch", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			ID   int64  `json:"id"`
			Type string `json:"type"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ID <= 0 {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"bad request body"}`))
			return
		}
		if body.Type == "" {
			body.Type = "animation"
		}
		t := hub.pushTask("fetch_keyframes", body.Type, body.ID)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"taskId": t.ID})
	})

	// POST /animreup/reupload , app initiates a single-id reupload via plugin
	// body: { id: number, type: "animation" | "sound" }
	// returns: { taskId: "..." }, caller polls /animreup/result for newId
	http.HandleFunc("POST /animreup/reupload", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			ID   int64  `json:"id"`
			Type string `json:"type"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ID <= 0 {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"bad request body"}`))
			return
		}
		if body.Type == "" {
			body.Type = "animation"
		}
		t := hub.pushTask("reupload", body.Type, body.ID)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"taskId": t.ID})
	})

	// GET /animreup/next-task , plugin long-polls (or short-polls) for the next task
	// blocks for up to 25s waiting for a task to arrive responds 204 on timeout
	// query: ?wait=ms (clamp 0..25000) default 25000 plugin sets wait=0 for short-poll fallback
	// every hit refreshes the liveness timestamp so /animreup/plugin-alive can
	// distinguish "plugin running" from "studio closed"
	http.HandleFunc("GET /animreup/next-task", func(w http.ResponseWriter, r *http.Request) {
		hub.markPolled()
		waitMs := 25000
		if q := r.URL.Query().Get("wait"); q != "" {
			var n int
			_, _ = fmt.Sscanf(q, "%d", &n)
			if n < 0 {
				n = 0
			} else if n > 25000 {
				n = 25000
			}
			waitMs = n
		}
		var t *animreupTask
		if waitMs == 0 {
			t = hub.claimNext()
		} else {
			t = hub.waitForTask(time.Duration(waitMs) * time.Millisecond)
		}
		if t == nil {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(t)
	})

	// GET /animreup/plugin-alive , app probes plugin presence
	// returns { alive: bool, lastPollMs: number } where lastPollMs is the age
	// of the most recent /next-task hit  app uses this to surface "plugin not
	// detected" up front instead of after a 30s task timeout
	http.HandleFunc("GET /animreup/plugin-alive", func(w http.ResponseWriter, r *http.Request) {
		age := hub.lastPollAge()
		// the plugin's long-poll holds for up to 25s + a request gap so
		// anything within 35s counts as "the poll loop is actively running"
		alive := age < 35*time.Second
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"alive":      alive,
			"lastPollMs": age.Milliseconds(),
		})
	})

	// GET /animreup/proxy-sound?id=N , server-side fetch of a roblox sound
	// asset  the direct asset-delivery url works fine over the wire but the
	// roblox cdn doesn't send access-control-allow-origin for tauri.localhost
	// so the browser blocks the response (you see "TypeError: Failed to fetch")
	// proxying through the backend sidesteps both csp and cors  errors and
	// status codes are passed through verbatim so the app can show a useful
	// failure message
	httpClient := &http.Client{Timeout: 30 * time.Second}
	http.HandleFunc("GET /animreup/proxy-sound", func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimSpace(r.URL.Query().Get("id"))
		if id == "" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"missing id"}`))
			return
		}
		upstream := "https://assetdelivery.roblox.com/v1/asset/?id=" + url.QueryEscape(id)
		req, err := http.NewRequestWithContext(r.Context(), "GET", upstream, nil)
		if err != nil {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		// roblox cdn likes a real-ish user agent  some endpoints 403 on the go
		// default ua so we paste a chrome string the same way studio does
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/120.0.0.0 Safari/537.36")
		resp, err := httpClient.Do(req)
		if err != nil {
			fmt.Println("animreup proxy-sound upstream err:", err)
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`{"error":"upstream fetch failed"}`))
			return
		}
		defer resp.Body.Close()
		// pass through the content-type and length so the renderer's blob
		// preserves the original mime  most roblox sounds are audio/ogg
		if ct := resp.Header.Get("Content-Type"); ct != "" {
			w.Header().Set("Content-Type", ct)
		}
		if cl := resp.Header.Get("Content-Length"); cl != "" {
			w.Header().Set("Content-Length", cl)
		}
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, resp.Body)
	})

	// POST /animreup/result/{taskId} , plugin posts the result blob
	// body: { error?: "..." } OR the result payload (free-form, app-typed)
	// the plugin sends e.g. { timeline: {...}, length, frameCount, rig, name } for
	// fetch_keyframes or { newId: ... } for reupload  errors come back as a top-level
	// error field
	http.HandleFunc("POST /animreup/result/{taskId}", func(w http.ResponseWriter, r *http.Request) {
		taskID := r.PathValue("taskId")
		if taskID == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			color.Error.Println("animreup result decode:", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		errMsg := ""
		if v, ok := payload["error"].(string); ok && strings.TrimSpace(v) != "" {
			errMsg = v
		}
		hub.storeResult(taskID, payload, errMsg)
		fmt.Printf("animreup: result stored for %s err=%q\n", taskID, errMsg)
		w.WriteHeader(http.StatusNoContent)
	})

	// GET /animreup/result/{taskId} , app polls for the result
	// returns: { pending: true } | { error: "..." } | the full result payload
	http.HandleFunc("GET /animreup/result/{taskId}", func(w http.ResponseWriter, r *http.Request) {
		taskID := r.PathValue("taskId")
		res := hub.getResult(taskID)
		if res == nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if !res.Done {
			_, _ = w.Write([]byte(`{"pending":true}`))
			return
		}
		if res.Error != "" {
			_ = json.NewEncoder(w).Encode(map[string]string{"error": res.Error})
			return
		}
		_ = json.NewEncoder(w).Encode(res.Payload)
	})

	fmt.Println("animreup routes registered")
}

// --- rbxdl-backed routes
// the animreup overlay uses three endpoints to do its job without the studio
// plugin:
//   POST /animreup/rbxdl-fetch       , metadata + downloadUrl for an asset id
//   GET  /animreup/rbxdl-proxy?id=N  , streams the raw file bytes (preview/download)
//   POST /animreup/rbxdl-reupload    , fetch via rbxdl + upload via publish.roblox.com
// all three share a single rbxdlClient so the laravel session-cookie + csrf
// token persist across calls (refreshing them costs a homepage roundtrip).

func setupAnimreupRbxdlRoutes(rbxdl *rbxdlClient, c *roblox.Client) {
	// POST /animreup/rbxdl-fetch , resolves an asset id to a signed download url
	// body: { id: number }
	// returns: { assetId, assetName, assetType, extension, size, downloadUrl }
	http.HandleFunc("POST /animreup/rbxdl-fetch", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			ID int64 `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ID <= 0 {
			writeJSONError(w, http.StatusBadRequest, "missing or invalid id")
			return
		}
		out, err := rbxdl.fetchAssetWithFallback(r.Context(), body.ID)
		if err != nil {
			color.Error.Println("rbxdl fetch:", err)
			writeJSONError(w, http.StatusBadGateway, err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(out)
	})

	// GET /animreup/rbxdl-proxy?id=N , streams the asset bytes via rbxdl. used by
	// the renderer for sound preview / download. for animations the response is
	// a binary .rbxm, the renderer just stores the blob and shows metadata.
	http.HandleFunc("GET /animreup/rbxdl-proxy", func(w http.ResponseWriter, r *http.Request) {
		idStr := strings.TrimSpace(r.URL.Query().Get("id"))
		if idStr == "" {
			writeJSONError(w, http.StatusBadRequest, "missing id")
			return
		}
		var assetID int64
		_, _ = fmt.Sscanf(idStr, "%d", &assetID)
		if assetID <= 0 {
			writeJSONError(w, http.StatusBadRequest, "invalid id")
			return
		}
		// pass the configured cookie if any so the assetdelivery fallback can
		// reach private / owned assets the rbxdl-signed url refuses.
		cookie := ""
		if c != nil {
			cookie = c.Cookie
		}
		meta, data, contentType, err := rbxdl.fetchAndDownload(r.Context(), assetID, cookie)
		if err != nil {
			color.Error.Println("rbxdl proxy:", err)
			writeJSONError(w, http.StatusBadGateway, err.Error())
			return
		}
		// pass through the upstream mime when present so the browser uses the
		// right codec hint for blob playback. fall back by extension otherwise.
		if contentType == "" {
			contentType = mimeForExtension(meta.Extension)
		}
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
		// expose the asset name + new metadata so the renderer can show it
		// without a separate fetch, saves a request and avoids ordering bugs.
		w.Header().Set("X-Asset-Name", meta.AssetName)
		w.Header().Set("X-Asset-Type", meta.AssetType)
		w.Header().Set("X-Asset-Extension", meta.Extension)
		w.Header().Set("X-Asset-Via", meta.Via)
		w.Header().Set("Access-Control-Expose-Headers", "X-Asset-Name, X-Asset-Type, X-Asset-Extension, X-Asset-Via")
		_, _ = w.Write(data)
	})

	// POST /animreup/rbxdl-reupload , fetches the asset via rbxdl, then uploads
	// it through publish.roblox.com (audio) or ide.roblox.com (animation) using
	// the currently-configured cookie. responds with { newId, oldId, name }.
	http.HandleFunc("POST /animreup/rbxdl-reupload", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			ID   int64  `json:"id"`
			Type string `json:"type"` // "sound" | "animation"
			Name string `json:"name"` // optional override; defaults to rbxdl-reported name
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ID <= 0 {
			writeJSONError(w, http.StatusBadRequest, "missing or invalid id")
			return
		}
		body.Type = strings.ToLower(strings.TrimSpace(body.Type))
		if body.Type == "" {
			body.Type = "sound"
		}
		if body.Type != "sound" && body.Type != "animation" {
			writeJSONError(w, http.StatusBadRequest, "type must be 'sound' or 'animation'")
			return
		}
		if c == nil || c.UserInfo.ID == 0 || c.Cookie == "" {
			writeJSONError(w, http.StatusUnauthorized, "no valid cookie set, add an account in re/up first")
			return
		}

		meta, data, _, err := rbxdl.fetchAndDownload(r.Context(), body.ID, c.Cookie)
		if err != nil {
			color.Error.Println("rbxdl reupload:", err)
			writeJSONError(w, http.StatusBadGateway, "rbxdl: "+err.Error())
			return
		}
		name := strings.TrimSpace(body.Name)
		if name == "" {
			name = strings.TrimSpace(meta.AssetName)
		}
		if name == "" {
			name = fmt.Sprintf("rbxdl-%d", body.ID)
		}

		switch body.Type {
		case "sound":
			handler, err := publish.NewUploadAudioHandler(c, name, bytes.NewBuffer(data))
			if err != nil {
				writeJSONError(w, http.StatusInternalServerError, "audio upload init: "+err.Error())
				return
			}
			// publish.NewUploadAudioHandler can fail twice in a row when the xsrf
			// token rotates, retry once on token-invalid so the first request of
			// a session doesn't die for nothing.
			resp, err := handler()
			if err != nil && err == publish.UploadAudioErrors.ErrTokenInvalid {
				resp, err = handler()
			}
			if err != nil {
				color.Error.Println("audio upload:", err)
				writeJSONError(w, http.StatusBadGateway, "audio upload: "+err.Error())
				return
			}
			if resp == nil || resp.ID == 0 {
				writeJSONError(w, http.StatusBadGateway, "audio upload: empty response")
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"oldId": body.ID,
				"newId": resp.ID,
				"name":  resp.Name,
				"via":   meta.Via,
			})
		case "animation":
			handler, err := ide.NewUploadAnimationHandler(c, name, "", bytes.NewBuffer(data))
			if err != nil {
				writeJSONError(w, http.StatusInternalServerError, "animation upload init: "+err.Error())
				return
			}
			newID, err := handler()
			if err != nil && err == ide.UploadAnimationErrors.ErrTokenInvalid {
				newID, err = handler()
			}
			if err != nil {
				color.Error.Println("animation upload:", err)
				writeJSONError(w, http.StatusBadGateway, "animation upload: "+err.Error())
				return
			}
			if newID == 0 {
				writeJSONError(w, http.StatusBadGateway, "animation upload: empty response")
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"oldId": body.ID,
				"newId": newID,
				"name":  name,
				"via":   meta.Via,
			})
		}
	})

	fmt.Println("animreup rbxdl routes registered")
}

func writeJSONError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func mimeForExtension(ext string) string {
	switch strings.ToLower(strings.TrimPrefix(ext, ".")) {
	case "ogg":
		return "audio/ogg"
	case "mp3":
		return "audio/mpeg"
	case "wav":
		return "audio/wav"
	case "png":
		return "image/png"
	case "jpg", "jpeg":
		return "image/jpeg"
	case "rbxm", "rbxmx":
		return "model/x-rbxm"
	case "rbxl", "rbxlx":
		return "model/x-rbxl"
	default:
		return "application/octet-stream"
	}
}
