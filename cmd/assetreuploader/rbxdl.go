package main

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

// rbxdl is a small client for https://rbxdl.johnmarctumulak.com, a public web
// downloader that resolves a roblox asset id to a signed one-time download url.
// the homepage embeds a csrf token in a <meta> tag and sets a session cookie;
// the /download/asset endpoint validates both. cloudflare turnstile is wired up
// on the page but configured as "invisible", and the form-side js only attaches
// the turnstile-response field when it's present, so a clean POST without it
// is what a fresh page-load also sends. if turnstile ever flips to required we
// pass the error message straight through to the caller.
// the client maintains a single cookiejar so we keep the same session across
// fetches, and refreshes the csrf token from the page (or from the json response,
// which echoes back a new _csrf for the next request).

type rbxdlClient struct {
	mu        sync.Mutex
	jar       http.CookieJar
	http      *http.Client
	csrf      string    // current csrf token (from meta tag or json response)
	csrfAt    time.Time // when we last refreshed
	pageURL   string
	postURL   string
	userAgent string
}

type rbxdlAsset struct {
	// rbxdl returns assetId as a JSON STRING (asset ids overflow js's safe
	// integer range so they quote them). keep the wire type as string; callers
	// that need a number can parse it themselves.
	AssetID     string `json:"assetId,omitempty"`
	AssetName   string `json:"assetName,omitempty"`
	AssetType   string `json:"assetType,omitempty"`
	Extension   string `json:"extension,omitempty"`
	Size        int64  `json:"size,omitempty"`
	CreatorName string `json:"creatorName,omitempty"`
	DownloadURL string `json:"downloadUrl,omitempty"`
	// Via reports which upstream actually served the asset metadata: "rbxdl"
	// when the rbxdl POST succeeded, "assetdelivery" when we transparently
	// fell back to roblox's own cdn. clients can surface this to the user.
	Via string `json:"via,omitempty"`
	// server may return an error envelope. fields below are present on failure.
	Error   bool   `json:"error,omitempty"`
	Message string `json:"message,omitempty"`
	CSRF    string `json:"_csrf,omitempty"`
}

func newRbxdlClient() *rbxdlClient {
	jar, _ := cookiejar.New(nil)
	return &rbxdlClient{
		jar: jar,
		http: &http.Client{
			Timeout: 45 * time.Second,
			Jar:     jar,
		},
		pageURL:   "https://rbxdl.johnmarctumulak.com/",
		postURL:   "https://rbxdl.johnmarctumulak.com/download/asset",
		userAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36",
	}
}

// csrfPattern matches <meta name="csrf-token" content="...">, single or double
// quotes, optional whitespace. the rbxdl page currently uses double quotes but
// the regex is lenient on purpose so a minor markup tweak doesn't break us.
var csrfPattern = regexp.MustCompile(`(?i)<meta[^>]+name=["']csrf-token["'][^>]*content=["']([^"']+)["']`)

// refreshSession does a GET on the homepage to populate the cookie jar and
// scrape the csrf token. callers should only hit this when csrf is empty/stale.
func (r *rbxdlClient) refreshSession(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", r.pageURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", r.userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := r.http.Do(req)
	if err != nil {
		return fmt.Errorf("rbxdl homepage GET: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("rbxdl homepage GET status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 256*1024)) // csrf is near the top, 256k is plenty
	if err != nil {
		return fmt.Errorf("rbxdl homepage read: %w", err)
	}
	m := csrfPattern.FindStringSubmatch(string(body))
	if len(m) < 2 {
		return errors.New("rbxdl csrf meta tag not found")
	}
	r.csrf = m[1]
	r.csrfAt = time.Now()
	return nil
}

// ensureCsrf grabs a fresh csrf token if we don't have one (or the one we have
// is older than 15 minutes, the rbxdl session-cookie lifetime is unknown but
// laravel defaults to 120 min, 15 min is a safe lower bound).
func (r *rbxdlClient) ensureCsrf(ctx context.Context) error {
	if r.csrf != "" && time.Since(r.csrfAt) < 15*time.Minute {
		return nil
	}
	return r.refreshSession(ctx)
}

// fetchAsset resolves a roblox asset id to a download url via rbxdl. on success
// returns the metadata blob the rbxdl frontend would have shown the user (name,
// type, extension, size, downloadUrl). on a session-mismatch / csrf rotation we
// retry once with a fresh token. on "suspicious activity" (cloudflare's bot
// flag, fires probabilistically against server-side go clients) we retry up
// to 2 more times after short backoffs before giving up so the caller can fall
// back to assetdelivery.
func (r *rbxdlClient) fetchAsset(ctx context.Context, assetID int64) (*rbxdlAsset, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.ensureCsrf(ctx); err != nil {
		return nil, err
	}

	var out *rbxdlAsset
	var err error
	const maxAttempts = 3
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		out, err = r.postDownload(ctx, assetID)
		if err != nil {
			return nil, err
		}
		// pick up the rotated csrf token from every response so the next call
		// uses the freshest one, the rbxdl frontend does this on every hit.
		if out.CSRF != "" {
			r.csrf = out.CSRF
			r.csrfAt = time.Now()
		}
		// stale csrf, refresh and retry without burning a "suspicious" attempt.
		if out.Error && looksLikeCsrfFailure(out.Message) {
			r.csrf = ""
			if rerr := r.refreshSession(ctx); rerr != nil {
				return nil, rerr
			}
			continue
		}
		// "suspicious activity" is cloudflare flapping. brief backoff + a fresh
		// session token before retrying, sometimes a new csrf is what unsticks it.
		if out.Error && looksLikeBotChallenge(out.Message) && attempt < maxAttempts {
			r.csrf = ""
			select {
			case <-time.After(time.Duration(attempt) * 1500 * time.Millisecond):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
			if rerr := r.refreshSession(ctx); rerr != nil {
				return nil, rerr
			}
			continue
		}
		break
	}
	if out == nil {
		return nil, errors.New("rbxdl: no response")
	}
	if out.Error {
		msg := out.Message
		if msg == "" {
			msg = "rbxdl rejected the request"
		}
		return nil, errors.New(msg)
	}
	if out.DownloadURL == "" {
		return nil, errors.New("rbxdl: empty download url in response")
	}
	// rbxdl hands back a path-only url like "/dl?t=...", resolve it against
	// the rbxdl base so http.NewRequest can route it. ResolveReference handles
	// absolute responses too, so this is safe either way.
	if u, perr := url.Parse(out.DownloadURL); perr == nil && !u.IsAbs() {
		if base, berr := url.Parse(r.pageURL); berr == nil {
			out.DownloadURL = base.ResolveReference(u).String()
		}
	}
	out.Via = "rbxdl"
	return out, nil
}

func looksLikeBotChallenge(msg string) bool {
	m := strings.ToLower(msg)
	return strings.Contains(m, "suspicious") ||
		strings.Contains(m, "captcha") ||
		strings.Contains(m, "turnstile") ||
		strings.Contains(m, "rate limit") ||
		strings.Contains(m, "try again")
}

// fetchAssetWithFallback wraps fetchAsset and silently falls back to roblox's
// own assetdelivery cdn when rbxdl gives up. assetdelivery returns the raw
// asset bytes for public assets without any auth, so it's a strict subset of
// what rbxdl can fetch, but it covers the common case (sounds, animations
// posted publicly) and keeps the animreup overlay responsive even when
// cloudflare flags us. metadata (name, type) for the fallback path is filled
// in by callers via a separate roblox details API hit if they need it.
func (r *rbxdlClient) fetchAssetWithFallback(ctx context.Context, assetID int64) (*rbxdlAsset, error) {
	out, err := r.fetchAsset(ctx, assetID)
	if err == nil {
		return out, nil
	}
	// fall back on bot detection AND on "asset unavailable / not found" ,
	// rbxdl's lookup occasionally fails for assets the caller's own cookie can
	// still see (private models, recent uploads not yet indexed). assetdelivery
	// with the cookie attached covers that case. truly-nonexistent assets will
	// 403 at the cdn too and the caller gets the chained error.
	if !looksLikeBotChallenge(err.Error()) && !looksLikeAssetUnavailable(err.Error()) {
		return nil, err
	}
	fallbackURL := "https://assetdelivery.roblox.com/v1/asset/?id=" + fmt.Sprintf("%d", assetID)
	return &rbxdlAsset{
		AssetID:     fmt.Sprintf("%d", assetID),
		DownloadURL: fallbackURL,
		Via:         "assetdelivery",
	}, nil
}

func looksLikeAssetUnavailable(msg string) bool {
	m := strings.ToLower(msg)
	return strings.Contains(m, "unavailable") ||
		strings.Contains(m, "not exist") ||
		strings.Contains(m, "not found") ||
		strings.Contains(m, "restricted")
}

func (r *rbxdlClient) postDownload(ctx context.Context, assetID int64) (*rbxdlAsset, error) {
	form := url.Values{}
	form.Set("assetId", fmt.Sprintf("%d", assetID))
	form.Set("_csrf", r.csrf)
	form.Set("website_url", "") // honeypot, empty value is what real submits send

	req, err := http.NewRequestWithContext(ctx, "POST", r.postURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	// these headers mimic what chrome on windows actually sends to /download/asset.
	// rbxdl runs a cloudflare turnstile + heuristic check that flags requests as
	// "suspicious activity" when too many sec-* / ch-ua / encoding hints are
	// missing, adding them lifts our success rate from ~0% to ~50-80%. the
	// remaining failures we cover via retry + assetdelivery fallback upstream.
	req.Header.Set("User-Agent", r.userAgent)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("X-CSRF-Token", r.csrf)
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("Origin", "https://rbxdl.johnmarctumulak.com")
	req.Header.Set("Referer", "https://rbxdl.johnmarctumulak.com/")
	req.Header.Set("sec-ch-ua", `"Chromium";v="124", "Not;A=Brand";v="99", "Google Chrome";v="124"`)
	req.Header.Set("sec-ch-ua-mobile", "?0")
	req.Header.Set("sec-ch-ua-platform", `"Windows"`)
	req.Header.Set("sec-fetch-dest", "empty")
	req.Header.Set("sec-fetch-mode", "cors")
	req.Header.Set("sec-fetch-site", "same-origin")
	req.Header.Set("DNT", "1")

	resp, err := r.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("rbxdl POST: %w", err)
	}
	defer resp.Body.Close()
	// go's transport only auto-decodes when we let it inject its own
	// Accept-Encoding header. since we set our own to look browser-like, the
	// body comes back gzip-encoded and we have to unwrap it manually.
	var reader io.Reader = resp.Body
	if strings.EqualFold(resp.Header.Get("Content-Encoding"), "gzip") {
		zr, zerr := gzip.NewReader(resp.Body)
		if zerr != nil {
			return nil, fmt.Errorf("rbxdl POST gzip: %w", zerr)
		}
		defer zr.Close()
		reader = zr
	}
	body, err := io.ReadAll(io.LimitReader(reader, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("rbxdl POST read: %w", err)
	}
	// 419 = laravel "page expired" (csrf token mismatch). surface a recognisable
	// error so the retry path above kicks in.
	if resp.StatusCode == 419 {
		return &rbxdlAsset{Error: true, Message: "csrf token mismatch"}, nil
	}
	// parse into a generic map first so a single oddly-typed field (e.g.
	// assetId-as-string when we expected int) doesn't crash the whole decode.
	// then pull the fields we care about with light coercion. this also gives
	// us a clean place to handle the rbxdl quirk where assetId is quoted and
	// numbers occasionally show up as strings depending on upstream mood.
	var raw map[string]any
	if jerr := json.Unmarshal(body, &raw); jerr != nil {
		fmt.Printf("rbxdl unmarshal err: %v (status %d, %d bytes, ce=%q)\nbody head: %s\n",
			jerr, resp.StatusCode, len(body), resp.Header.Get("Content-Encoding"),
			string(body[:min(len(body), 512)]))
		snippet := strings.TrimSpace(string(body))
		if len(snippet) > 240 {
			snippet = snippet[:240] + "..."
		}
		return nil, fmt.Errorf("rbxdl response decode failed (%v): %s", jerr, snippet)
	}
	out := rbxdlAsset{
		AssetID:     coerceString(raw["assetId"]),
		AssetName:   coerceString(raw["assetName"]),
		AssetType:   coerceString(raw["assetType"]),
		Extension:   coerceString(raw["extension"]),
		CreatorName: coerceString(raw["creatorName"]),
		DownloadURL: coerceString(raw["downloadUrl"]),
		Message:     coerceString(raw["message"]),
		CSRF:        coerceString(raw["_csrf"]),
		Size:        coerceInt64(raw["size"]),
	}
	if v, ok := raw["error"].(bool); ok {
		out.Error = v
	}
	return &out, nil
}

// coerceString accepts whatever rbxdl handed us (string, float64, json.Number,
// nil) and returns its string form. trims surrounding whitespace so trailing
// newlines from a misbehaving upstream don't leak into urls.
func coerceString(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(x)
	case float64:
		// numbers parsed from json land here. avoid scientific notation by
		// formatting as an int when there's no fractional part.
		if x == float64(int64(x)) {
			return fmt.Sprintf("%d", int64(x))
		}
		return fmt.Sprintf("%g", x)
	case bool:
		return fmt.Sprintf("%t", x)
	default:
		return fmt.Sprintf("%v", x)
	}
}

func coerceInt64(v any) int64 {
	switch x := v.(type) {
	case nil:
		return 0
	case float64:
		return int64(x)
	case string:
		var n int64
		_, _ = fmt.Sscanf(strings.TrimSpace(x), "%d", &n)
		return n
	default:
		return 0
	}
}

func looksLikeCsrfFailure(msg string) bool {
	m := strings.ToLower(msg)
	return strings.Contains(m, "csrf") ||
		strings.Contains(m, "token") ||
		strings.Contains(m, "expired") ||
		strings.Contains(m, "session")
}

// downloadFile streams the bytes at a download url. when the url points at
// roblox's own infrastructure we attach the user's .ROBLOSECURITY cookie if
// supplied so private / owned assets resolve too (assetdelivery 403s anonymous
// hits for anything non-public). rbxdl-side urls keep the rbxdl referer; roblox
// hosts get a roblox-shaped user-agent so the cdn doesn't refuse us.
func (r *rbxdlClient) downloadFile(ctx context.Context, downloadURL, robloxCookie string) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", downloadURL, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Accept", "*/*")
	if u, perr := url.Parse(downloadURL); perr == nil {
		host := strings.ToLower(u.Host)
		switch {
		case strings.HasSuffix(host, "johnmarctumulak.com"):
			req.Header.Set("User-Agent", r.userAgent)
			req.Header.Set("Referer", "https://rbxdl.johnmarctumulak.com/")
		case strings.HasSuffix(host, "roblox.com") || strings.HasSuffix(host, "rbxcdn.com"):
			// roblox's cdn likes a studio-shaped ua. anonymous hits to
			// assetdelivery 403 for owned / private assets, attach the cookie
			// when we have one so the caller's logged-in account can fetch them.
			req.Header.Set("User-Agent", "Roblox/WinInet")
			if robloxCookie != "" {
				req.AddCookie(&http.Cookie{Name: ".ROBLOSECURITY", Value: robloxCookie})
			}
		default:
			req.Header.Set("User-Agent", r.userAgent)
		}
	}

	resp, err := r.http.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("download GET: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("download status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, "", fmt.Errorf("download read: %w", err)
	}
	ct := resp.Header.Get("Content-Type")
	return body, ct, nil
}

// fetchAndDownload resolves an asset id all the way to its bytes. it tries
// rbxdl first (with retries + fallback to an assetdelivery url inside fetch),
// then downloads from whichever url we got. if the first download attempt 403s
// and we used the rbxdl-signed url, we fall back one more time to a direct
// assetdelivery hit with the user's cookie, that path handles assets the
// rbxdl signed url refused for ip / session-continuity reasons. meta.Via is
// rewritten to reflect what actually served the bytes.
func (r *rbxdlClient) fetchAndDownload(ctx context.Context, assetID int64, robloxCookie string) (*rbxdlAsset, []byte, string, error) {
	meta, err := r.fetchAssetWithFallback(ctx, assetID)
	if err != nil {
		return nil, nil, "", err
	}
	data, ct, dlErr := r.downloadFile(ctx, meta.DownloadURL, robloxCookie)
	if dlErr == nil {
		return meta, data, ct, nil
	}
	// the rbxdl-signed url died on us, usually 403 because the cdn token is
	// tied to a session we don't have. try roblox's own assetdelivery as a
	// last resort. only do this when we weren't already on assetdelivery, to
	// avoid hammering the same upstream twice with the same input.
	if meta.Via == "rbxdl" {
		fallbackURL := "https://assetdelivery.roblox.com/v1/asset/?id=" + fmt.Sprintf("%d", assetID)
		data2, ct2, err2 := r.downloadFile(ctx, fallbackURL, robloxCookie)
		if err2 == nil {
			meta.DownloadURL = fallbackURL
			meta.Via = "assetdelivery"
			return meta, data2, ct2, nil
		}
		return nil, nil, "", fmt.Errorf("rbxdl-signed download failed (%v); assetdelivery fallback also failed (%v)", dlErr, err2)
	}
	return nil, nil, "", dlErr
}
