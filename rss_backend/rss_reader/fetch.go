package rssreader

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

// fetchURL fetches the content from the provided URL, enforcing the configured body size limit.
func (r *rssReader) fetchURL(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "rssreader/1.0")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected HTTP status: %d %s", resp.StatusCode, resp.Status)
	}

	// Read up to maxBodySize + 1 bytes to detect truncation.
	limitedReader := io.LimitReader(resp.Body, r.cfg.maxBodySize+1)
	body, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if int64(len(body)) > r.cfg.maxBodySize {
		return nil, fmt.Errorf("response body exceeds maximum size of %d bytes", r.cfg.maxBodySize)
	}

	return body, nil
}
