package globalTypes

import (
	"context"
	"net/http"
	"time"
)

func urlIsDownloadable(ctx context.Context, url string) (ok bool, status int, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false, 0, err
	}
	req.Header.Set("Range", "bytes=0-0")          // 1 Byte
	req.Header.Set("User-Agent", "url-check/1.0") // hilft bei manchen Hosts

	c := &http.Client{Timeout: 10 * time.Second}
	resp, err := c.Do(req)
	if err != nil {
		return false, 0, err
	}
	defer resp.Body.Close()

	status = resp.StatusCode

	// 200 = Server ignoriert Range, 206 = Partial Content (Range ok)
	if status == http.StatusOK || status == http.StatusPartialContent {
		return true, status, nil
	}
	return false, status, nil
}
