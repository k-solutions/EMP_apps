package worker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/emarchant/rssreader"
	"github.com/emarchant/rssservice/internal/cache"
	"github.com/emarchant/rssservice/internal/jobstore"
	"github.com/emarchant/rssservice/internal/rabbitmq"
	"github.com/redis/go-redis/v9"
)

type Processor struct {
	reader    rssreader.RssReader
	store     jobstore.JobStore
	rdb       *redis.Client
	urlCache  *cache.URLCache
	publisher *rabbitmq.Publisher
	logger    *slog.Logger
}

func NewProcessor(
	reader rssreader.RssReader,
	store jobstore.JobStore,
	rdb *redis.Client,
	urlCache *cache.URLCache,
	publisher *rabbitmq.Publisher,
	logger *slog.Logger,
) *Processor {
	return &Processor{
		reader:    reader,
		store:     store,
		rdb:       rdb,
		urlCache:  urlCache,
		publisher: publisher,
		logger:    logger,
	}
}

func (p *Processor) Process(ctx context.Context, jobID string, urls []string) error {
	p.logger.Info("started processing job", "job_id", jobID, "url_count", len(urls))
	startTime := time.Now()

	// 1. Update jobstore status to processing
	job := &jobstore.Job{
		ID:     jobID,
		Status: "processing",
	}
	if err := p.store.Update(ctx, job); err != nil {
		p.logger.Error("failed to update job status to processing", "job_id", jobID, "err", err)
		return fmt.Errorf("failed to update job status: %w", err)
	}

	// 2. Parse RSS/Atom feeds using transient cache where possible
	var finalItems []jobstore.RssItem
	var finalErrors []jobstore.URLErr
	var urlsToFetch []string

	// Check cache first for each URL
	for _, url := range urls {
		cachedItems, err := p.urlCache.Get(ctx, url)
		if err == nil {
			p.logger.Debug("cache hit for URL", "url", url, "job_id", jobID, "items", len(cachedItems))
			finalItems = append(finalItems, cachedItems...)
		} else {
			p.logger.Debug("cache miss for URL", "url", url, "job_id", jobID)
			urlsToFetch = append(urlsToFetch, url)
		}
	}

	// If there are missed URLs, fetch them
	if len(urlsToFetch) > 0 {
		parsedItems, parseErr := p.reader.Parse(ctx, urlsToFetch)

		// Group parsed items by their SourceURL for atomic cache hydration
		fetchedItemsByURL := make(map[string][]jobstore.RssItem)
		for _, item := range parsedItems {
			rssItem := jobstore.RssItem{
				Title:       item.Title,
				Source:      item.Source,
				SourceURL:   item.SourceURL,
				Link:        item.Link,
				PublishDate: item.PublishDate.String(),
				Description: item.Description,
			}
			finalItems = append(finalItems, rssItem)
			fetchedItemsByURL[item.SourceURL] = append(fetchedItemsByURL[item.SourceURL], rssItem)
		}

		// Handle errors
		var urlErrs []jobstore.URLErr

		if parseErr != nil {
			var pErr rssreader.ParseError
			if errors.As(parseErr, &pErr) {
				for _, ue := range pErr.Errors() {
					urlErr := jobstore.URLErr{
						URL:   ue.URL(),
						Error: ue.Error(),
					}
					urlErrs = append(urlErrs, urlErr)
					finalErrors = append(finalErrors, urlErr)
				}
			} else {
				for _, u := range urlsToFetch {
					urlErr := jobstore.URLErr{
						URL:   u,
						Error: parseErr.Error(),
					}
					urlErrs = append(urlErrs, urlErr)
					finalErrors = append(finalErrors, urlErr)
				}
			}
		}

		// Hydrate Cache for successful fetches
		for _, u := range urlsToFetch {
			hasFailed := false
			for _, ue := range urlErrs {
				if ue.URL == u {
					hasFailed = true
					break
				}
			}
			if !hasFailed {
				itemsForURL := fetchedItemsByURL[u]
				if itemsForURL == nil {
					itemsForURL = []jobstore.RssItem{}
				}
				if err := p.urlCache.Set(ctx, u, itemsForURL); err != nil {
					p.logger.Error("failed to hydrate cache", "url", u, "err", err)
				}
			}
		}

	}

	allFailed := len(finalItems) == 0 && len(finalErrors) >= len(urls) && len(urls) > 0
	status := "done"
	if allFailed {
		status = "failed"
	}

	// 3. Update Job Store with final results
	job.Status = status
	if allFailed {
		job.Error = "all URLs failed"
		job.Items = nil
		job.Errors = finalErrors
	} else {
		job.Items = finalItems
		job.Errors = finalErrors
	}

	if err := p.store.Update(ctx, job); err != nil {
		p.logger.Error("failed to save final job status", "job_id", jobID, "err", err)
		return fmt.Errorf("failed to save final job status: %w", err)
	}

	// 4. Publish to results exchange if publisher is available
	durationMs := time.Since(startTime).Milliseconds()
	if p.publisher != nil {
		err := p.publisher.PublishResult(ctx, jobID, status, job.Items, job.Errors)
		if err != nil {
			p.logger.Error("failed to publish result event to RabbitMQ", "job_id", jobID, "err", err)
			return fmt.Errorf("failed to publish result: %w", err)
		}
	}

	p.logger.Info("completed processing job",
		"job_id", jobID,
		"status", status,
		"url_count", len(urls),
		"item_count", len(finalItems),
		"error_count", len(finalErrors),
		"duration_ms", durationMs,
	)

	return nil
}
