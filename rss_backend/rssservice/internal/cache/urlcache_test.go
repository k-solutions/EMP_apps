package cache

import (
	"context"
	"testing"
	"time"

	"github.com/emarchant/rssservice/internal/jobstore"
	"github.com/redis/go-redis/v9"
)

func TestURLCacheFingerprint(t *testing.T) {
	uc := NewURLCache(nil, 1*time.Hour)
	url := "https://feeds.bbci.co.uk/news/rss.xml"
	key := uc.Fingerprint(url)

	expectedPrefix := "cache:url:"
	if len(key) != len(expectedPrefix)+32 {
		t.Errorf("expected key of length %d, got %d", len(expectedPrefix)+32, len(key))
	}
	if key[:len(expectedPrefix)] != expectedPrefix {
		t.Errorf("expected prefix %q, got %q", expectedPrefix, key[:len(expectedPrefix)])
	}
}

func TestURLCacheNilClient(t *testing.T) {
	uc := NewURLCache(nil, 1*time.Hour)
	ctx := context.Background()
	url := "https://feeds.bbci.co.uk/news/rss.xml"

	_, err := uc.Get(ctx, url)
	if err != redis.Nil {
		t.Errorf("expected redis.Nil on nil client Get, got %v", err)
	}

	err = uc.Set(ctx, url, []jobstore.RssItem{})
	if err != nil {
		t.Errorf("expected nil error on nil client Set, got %v", err)
	}
}

func TestURLCacheWithRedis(t *testing.T) {
	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Skip("skipping URL Cache integration test: local Redis not running")
	}
	defer rdb.Close()

	uc := NewURLCache(rdb, 5*time.Second)
	url := "https://feeds.bbci.co.uk/news/rss.xml"

	// Cleanup
	rdb.Del(ctx, uc.Fingerprint(url))
	defer rdb.Del(ctx, uc.Fingerprint(url))

	// Get should fail on cache miss
	_, err := uc.Get(ctx, url)
	if err != redis.Nil {
		t.Fatalf("expected cache miss redis.Nil, got %v", err)
	}

	// Set cached items
	items := []jobstore.RssItem{
		{
			Title:     "Sample Article",
			Source:     "BBC News",
			SourceURL: url,
			Link:      "https://example.com/item",
		},
	}
	err = uc.Set(ctx, url, items)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Get should hit
	cachedItems, err := uc.Get(ctx, url)
	if err != nil {
		t.Fatalf("Get after Set failed: %v", err)
	}

	if len(cachedItems) != 1 || cachedItems[0].Title != "Sample Article" {
		t.Errorf("retrieved cached items mismatch: %+v", cachedItems)
	}
}
