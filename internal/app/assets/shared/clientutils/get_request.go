package clientutils

import (
	"bytes"
	"errors"
	"io"
	"net/http"

	"github.com/Obey72/backend/internal/retry"
	"github.com/Obey72/backend/internal/roblox"
)

func GetRequest(c *roblox.Client, url string) (*bytes.Buffer, error) {
	req, err := http.NewRequest("GET", url, http.NoBody)
	if err != nil {
		return nil, err
	}

	body, err := retry.Do(
		retry.NewOptions(retry.Tries(3)),
		func(_ int) (*bytes.Buffer, error) {
			resp, err := c.DoRequest(req)
			if err != nil {
				return nil, &retry.ContinueRetry{Err: err}
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				return nil, &retry.ExitRetry{Err: errors.New(resp.Status)}
			}

			var buffer bytes.Buffer
			io.Copy(&buffer, resp.Body)
			return &buffer, nil
		},
	)
	if err != nil {
		return nil, err
	}

	return body, nil
}
