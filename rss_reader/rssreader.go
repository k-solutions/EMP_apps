package rssreader

import (
	"context"
	"fmt"
	"sync"
)

// RssItem represents an individual item parsed from an RSS or Atom feed.
type RssItem struct {
	Title       string
	Source      string // Feed / channel title
	SourceURL   string // URL the feed was fetched from
	Link        string // URL of the individual post (dedup key)
	PublishDate Date   // Calendar date only — no time or timezone
	Description string
}

// RssReader is the public interface to parse RSS/Atom feeds.
type RssReader interface {
	// Parse fetches and parses all provided URLs concurrently.
	// ctx controls cancellation and timeouts.
	// Partial results are returned alongside a *ParseErrors when some URLs fail.
	Parse(ctx context.Context, urls []string) ([]RssItem, error)
}

type rssReader struct {
	cfg config
}

// New returns a new RssReader with the given options applied.
func New(opts ...Option) RssReader {
	cfg := config{
		concurrencyLimit: defaultConcurrencyLimit,
		maxBodySize:      defaultMaxBodySize,
	}
	for _, o := range opts {
		o(&cfg)
	}
	return &rssReader{cfg: cfg}
}

// Parse fetches and parses the feeds from the provided URLs in parallel.
// ctx is passed through to every HTTP request, so callers control timeouts:
//
//	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
//	defer cancel()
//	items, err := reader.Parse(ctx, urls)
func (r *rssReader) Parse(ctx context.Context, urls []string) ([]RssItem, error) {
	if len(urls) == 0 {
		return []RssItem{}, nil
	}

	sem := make(chan struct{}, r.cfg.concurrencyLimit)

	type result struct {
		url   string
		items []RssItem
		err   error
	}

	resultsChan := make(chan result, len(urls))
	var wg sync.WaitGroup

	for _, url := range urls {
		wg.Add(1)
		go func(u string) {
			defer wg.Done()

			// Respect cancellation before even acquiring the semaphore.
			select {
			case <-ctx.Done():
				resultsChan <- result{url: u, err: ctx.Err()}
				return
			case sem <- struct{}{}:
			}
			defer func() { <-sem }()

			items, err := r.fetchAndParse(ctx, u)
			resultsChan <- result{url: u, items: items, err: err}
		}(url)
	}

	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	var allItems []RssItem
	var urlErrs []URLError

	for res := range resultsChan {
		if res.err != nil {
			urlErrs = append(urlErrs, newURLError(res.url, res.err))
		} else {
			allItems = append(allItems, res.items...)
		}
	}

	items := deduplicate(allItems)
	sortItems(items)

	return items, newParseErrors(urlErrs)
}

// fetchAndParse fetches a single URL and tries RSS then Atom parsing.
// Trying both parsers in order is simpler than format detection and avoids
// duplicated fallback blocks.
func (r *rssReader) fetchAndParse(ctx context.Context, url string) ([]RssItem, error) {
	data, err := r.fetchURL(ctx, url)
	if err != nil {
		return nil, err
	}

	if items, err := parseRSS(data, url); err == nil {
		return items, nil
	}

	if items, err := parseAtom(data, url); err == nil {
		return items, nil
	}

	return nil, fmt.Errorf("unable to parse feed at %s: unsupported or malformed format", url)
}
