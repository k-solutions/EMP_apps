//go:build integration

package rssreader

import (
	"context"
	"testing"
	"time"
)

func TestIntegration(t *testing.T) {
	urls := []string{
		"https://news.ycombinator.com/rss",
		"https://go.dev/blog/feed.atom",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	reader := New()
	items, err := reader.Parse(ctx, urls)
	if err != nil {
		t.Logf("Error/partial error during integration fetch (could be network or external failure): %v", err)
	}

	if len(items) == 0 {
		t.Fatalf("expected to parse items from public feeds, got 0")
	}

	t.Logf("Successfully fetched and parsed %d items from Hacker News and Go Blog", len(items))

	// Assert at least some items have valid publish dates
	var validDates int
	for _, item := range items {
		if !item.PublishDate.IsZero() {
			validDates++
		}
	}

	if validDates == 0 {
		t.Errorf("expected at least some parsed items to have non-zero publish dates")
	}
}
