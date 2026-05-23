package roblox

import (
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"
)

// roblox sets this prefix on every roblosecurity cookie via set-cookie
// browsers strip it when copying so users paste a bare cookie that the api accepts
// the upload api however parses the cookie out of the prefixed form so we re-add it
const cookieWarning = "WARNING:-DO-NOT-SHARE-THIS.--Sharing-this-will-allow-someone-to-log-in-as-you-and-to-steal-your-ROBUX-and-items."
const cookiePrefix = "_|" + cookieWarning + "|_"

var (
	ErrEmptyCookie = errors.New("cookie is empty")
)

type Client struct {
	Cookie   string
	UserInfo UserInfo

	httpClient *http.Client

	token      string
	tokenMutex sync.RWMutex
}

func NewClient(cookie string) (*Client, error) {
	c := &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	if err := c.SetCookie(cookie); err != nil {
		return c, err
	}

	return c, nil
}

// auto prepend the warning prefix if missing otherwise users have to
// hand-construct the full string and most don't know that's required
func normalizeCookie(cookie string) string {
	cookie = strings.TrimSpace(cookie)
	if cookie == "" {
		return ""
	}
	if strings.Contains(cookie, cookieWarning) {
		return cookie
	}
	return cookiePrefix + cookie
}

func (c *Client) SetCookie(cookie string) error {
	normalized := normalizeCookie(cookie)
	if normalized == "" {
		c.Cookie = ""
		return ErrEmptyCookie
	}
	c.Cookie = normalized

	userInfo, err := authenticate(c, normalized)
	if err != nil {
		return err
	}

	c.UserInfo = userInfo
	return nil
}

func (c *Client) GetToken() string {
	c.tokenMutex.RLock()
	defer c.tokenMutex.RUnlock()
	return c.token
}

func (c *Client) SetToken(s string) {
	c.tokenMutex.Lock()
	c.token = s
	c.tokenMutex.Unlock()
}

func (c *Client) DoRequest(req *http.Request) (*http.Response, error) {
	return c.httpClient.Do(req)
}
