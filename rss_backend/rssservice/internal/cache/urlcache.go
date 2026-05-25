package cache

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/emarchant/rssservice/internal/jobstore"
	"github.com/redis/go-redis/v9"
)

type URLCache struct {
	client *redis.Client
	ttl    time.Duration
}

func NewURLCache(client *redis.Client, ttl time.Duration) *URLCache {
	return &URLCache{
		client: client,
		ttl:    ttl,
	}
}

// Fingerprint returns the cache key for a given URL.
func (c *URLCache) Fingerprint(url string) string {
	hash := md5.Sum([]byte(url))
	return "cache:url:" + hex.EncodeToString(hash[:])
}

// Get retrieves cached RSS items for a given URL.
func (c *URLCache) Get(ctx context.Context, url string) ([]jobstore.RssItem, error) {
	if c.client == nil {
		return nil, redis.Nil
	}
	key := c.Fingerprint(url)
	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		return nil, err
	}

	var items []jobstore.RssItem
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, err
	}
	return items, nil
}

// Set stores RSS items for a given URL with the configured TTL.
func (c *URLCache) Set(ctx context.Context, url string, items []jobstore.RssItem) error {
	if c.client == nil {
		return nil
	}
	key := c.Fingerprint(url)
	data, err := json.Marshal(items)
	if err != nil {
		return err
	}
	return c.client.Set(ctx, key, data, c.ttl).Err()
}
