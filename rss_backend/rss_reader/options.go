package rssreader

import (
	"sort"
)

const (
	defaultConcurrencyLimit = 10
	defaultMaxBodySize      = 10 * 1024 * 1024 // 10 MB
)

type config struct {
	concurrencyLimit int
	maxBodySize      int64
}

// Option configures an RssReader.
type Option func(*config)

// WithConcurrencyLimit sets the maximum number of URLs fetched in parallel.
func WithConcurrencyLimit(limit int) Option {
	return func(c *config) {
		if limit > 0 {
			c.concurrencyLimit = limit
		}
	}
}

// WithMaxBodySize sets the maximum response body size in bytes.
func WithMaxBodySize(size int64) Option {
	return func(c *config) {
		if size > 0 {
			c.maxBodySize = size
		}
	}
}

// deduplicate removes items with duplicate Link values, keeping the first
// occurrence. Items with an empty Link are always kept.
func deduplicate(items []RssItem) []RssItem {
	seen := make(map[string]struct{}, len(items))
	out := make([]RssItem, 0, len(items))

	for _, item := range items {
		if item.Link == "" {
			out = append(out, item)
			continue
		}
		if _, exists := seen[item.Link]; !exists {
			seen[item.Link] = struct{}{}
			out = append(out, item)
		}
	}

	return out
}

// sortItems sorts items by PublishDate descending (newest first).
// Items with a zero Date sort to the end.
func sortItems(items []RssItem) {
	sort.Slice(items, func(i, j int) bool {
		di, dj := items[i].PublishDate, items[j].PublishDate
		if di.IsZero() {
			return false
		}
		if dj.IsZero() {
			return true
		}
		return di.After(dj)
	})
}
