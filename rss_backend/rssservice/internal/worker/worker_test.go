package worker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/emarchant/rssreader"
	"github.com/emarchant/rssservice/internal/cache"
	"github.com/emarchant/rssservice/internal/jobstore"
	"github.com/redis/go-redis/v9"
)

type mockRssReader struct {
	items      []rssreader.RssItem
	err        error
	parseCalls int
	parsedURLs []string
}

func (m *mockRssReader) Parse(ctx context.Context, urls []string) ([]rssreader.RssItem, error) {
	m.parseCalls++
	m.parsedURLs = append(m.parsedURLs, urls...)
	return m.items, m.err
}

type mockURLError struct {
	url string
	err error
}

func (e mockURLError) Error() string { return fmt.Sprintf("%s: %v", e.url, e.err) }
func (e mockURLError) URL() string   { return e.url }
func (e mockURLError) Unwrap() error { return e.err }

type mockParseError struct {
	errs []rssreader.URLError
}

func (e mockParseError) Error() string                { return "parse errors" }
func (e mockParseError) Errors() []rssreader.URLError { return e.errs }
func (e mockParseError) Unwrap() []error {
	var errs []error
	for _, ue := range e.errs {
		errs = append(errs, ue)
	}
	return errs
}

func TestProcessorHappyPath(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	store := jobstore.NewInMemoryJobStore()

	date := rssreader.Date{Year: 2026, Month: time.May, Day: 24}
	mockItems := []rssreader.RssItem{
		{
			Title:       "Test Article",
			Source:      "Test Feed",
			SourceURL:   "https://example.com/feed",
			Link:        "https://example.com/article",
			PublishDate: date,
			Description: "A description",
		},
	}
	reader := &mockRssReader{items: mockItems, err: nil}
	urlCache := cache.NewURLCache(nil, 1*time.Hour)

	processor := NewProcessor(reader, store, nil, urlCache, nil, logger)

	ctx := context.Background()
	jobID := "job-happy-1"
	err := store.Create(ctx, &jobstore.Job{
		ID:     jobID,
		Status: "pending",
	})
	if err != nil {
		t.Fatalf("failed to create job: %v", err)
	}

	err = processor.Process(ctx, jobID, []string{"https://example.com/feed"})
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	job, err := store.Get(ctx, jobID)
	if err != nil {
		t.Fatalf("failed to retrieve job: %v", err)
	}

	if job.Status != "done" {
		t.Errorf("expected status = done, got %s", job.Status)
	}
	if len(job.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(job.Items))
	}
	item := job.Items[0]
	if item.Title != "Test Article" || item.PublishDate != "2026-05-24" {
		t.Errorf("unexpected item details: %+v", item)
	}
}

func TestProcessorPartialFailure(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	store := jobstore.NewInMemoryJobStore()

	date := rssreader.Date{Year: 2026, Month: time.May, Day: 24}
	mockItems := []rssreader.RssItem{
		{
			Title:       "Valid Article",
			Source:      "Valid Feed",
			SourceURL:   "https://example.com/valid",
			Link:        "https://example.com/article",
			PublishDate: date,
		},
	}
	mockErr := mockParseError{
		errs: []rssreader.URLError{
			mockURLError{url: "https://example.com/bad", err: errors.New("connection reset")},
		},
	}
	reader := &mockRssReader{items: mockItems, err: mockErr}
	urlCache := cache.NewURLCache(nil, 1*time.Hour)

	processor := NewProcessor(reader, store, nil, urlCache, nil, logger)

	ctx := context.Background()
	jobID := "job-partial-1"
	store.Create(ctx, &jobstore.Job{ID: jobID, Status: "pending"})

	err := processor.Process(ctx, jobID, []string{"https://example.com/valid", "https://example.com/bad"})
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	job, _ := store.Get(ctx, jobID)
	if job.Status != "done" {
		t.Errorf("expected status = done for partial failure, got %s", job.Status)
	}
	if len(job.Items) != 1 {
		t.Errorf("expected 1 item, got %d", len(job.Items))
	}
	if len(job.Errors) != 1 {
		t.Errorf("expected 1 URL error, got %d", len(job.Errors))
	}
	if job.Errors[0].URL != "https://example.com/bad" {
		t.Errorf("unexpected error item: %+v", job.Errors[0])
	}
}

func TestProcessorTotalFailure(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	store := jobstore.NewInMemoryJobStore()

	mockErr := mockParseError{
		errs: []rssreader.URLError{
			mockURLError{url: "https://example.com/bad", err: errors.New("timeout")},
		},
	}
	reader := &mockRssReader{items: nil, err: mockErr}
	urlCache := cache.NewURLCache(nil, 1*time.Hour)

	processor := NewProcessor(reader, store, nil, urlCache, nil, logger)

	ctx := context.Background()
	jobID := "job-total-1"
	store.Create(ctx, &jobstore.Job{ID: jobID, Status: "pending"})

	err := processor.Process(ctx, jobID, []string{"https://example.com/bad"})
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	job, _ := store.Get(ctx, jobID)
	if job.Status != "failed" {
		t.Errorf("expected status = failed for total failure, got %s", job.Status)
	}
	if len(job.Items) != 0 {
		t.Errorf("expected 0 items, got %d", len(job.Items))
	}
	if len(job.Errors) != 1 {
		t.Errorf("expected 1 URL error, got %d", len(job.Errors))
	}
}

func TestProcessorCachingSuppressOutbound(t *testing.T) {
	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Skip("skipping Processor caching test: local Redis not running")
	}
	defer rdb.Close()

	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	store := jobstore.NewInMemoryJobStore()
	urlCache := cache.NewURLCache(rdb, 10*time.Second)

	url := "https://example.com/cached-feed"
	rdb.Del(ctx, urlCache.Fingerprint(url))
	defer rdb.Del(ctx, urlCache.Fingerprint(url))

	// Pre-populate cache directly
	cachedItems := []jobstore.RssItem{
		{
			Title:     "Cached Article",
			Source:     "Cached Source",
			SourceURL: url,
			Link:      "https://example.com/item",
		},
	}
	_ = urlCache.Set(ctx, url, cachedItems)

	// mockReader should NOT be called at all
	reader := &mockRssReader{}
	processor := NewProcessor(reader, store, rdb, urlCache, nil, logger)

	jobID := "job-caching-1"
	_ = store.Create(ctx, &jobstore.Job{ID: jobID, Status: "pending"})

	err := processor.Process(ctx, jobID, []string{url})
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	// Verify reader was NOT called
	if reader.parseCalls != 0 {
		t.Errorf("expected reader to not be called (cache hit), but was called %d times", reader.parseCalls)
	}

	// Verify job in store got populated with cached items
	job, _ := store.Get(ctx, jobID)
	if job.Status != "done" {
		t.Errorf("expected job status 'done', got %s", job.Status)
	}
	if len(job.Items) != 1 || job.Items[0].Title != "Cached Article" {
		t.Errorf("expected cached items to be used, got: %+v", job.Items)
	}
}
