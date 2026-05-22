# RSS Reader

A lightweight, concurrent, and robust RSS 2.0 and Atom feed reader for Go.

[![Go Reference](https://pkg.go.dev/badge/github.com/emarchant/rssreader.svg)](https://pkg.go.dev/github.com/emarchant/rssreader)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

## Features

- **Sequential Try-Parsing**: Simple, sequence-based format matching (tries RSS 2.0, then Atom) rather than fragile auto-detection.
- **Concurrent Fetching**: Fetches multiple URLs concurrently, managed by a configurable semaphore to prevent socket exhaustion.
- **Robust Error Handling**: Collects errors per URL, returning successful results **alongside** partial failures using the custom `GroupedErrors` type.
- **Deduplication**: Automatically deduplicates articles by their canonical `Link` across all feeds.
- **Calendar Dates**: Custom `Date` type without times or timezones ensuring predictable, pure calendar-day handling.
- **Sorted Output**: Returns parsed items sorted by `PublishDate` descending (newest first). Zero dates sort last.
- **Pure Standard Library**: Zero external dependencies.

---

## Installation

```bash
go get github.com/emarchant/rssreader
```

---

## Quick Start

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/emarchant/rssreader"
)

func main() {
	urls := []string{
		"https://news.ycombinator.com/rss",
		"https://go.dev/blog/feed.atom",
	}

	// 1. Instantiate the reader
	reader := rssreader.New()

	// 2. Fetch and parse concurrently with a context timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	items, err := reader.Parse(ctx, urls)
	if err != nil {
		log.Printf("Some feed fetches failed: %v", err)
	}

	// 3. Print parsed items
	for _, item := range items {
		fmt.Printf("[%s] %s\n  Link:   %s\n  Source: %s\n\n",
			item.PublishDate, // formats automatically as "YYYY-MM-DD"
			item.Title,
			item.Link,
			item.Source,
		)
	}
}
```

---

## Configuration

`New` accepts functional options to customize fetching parameters:

```go
// Create a reader with a concurrency limit of 5 and max body size of 5 MB
reader := rssreader.New(
	rssreader.WithConcurrencyLimit(5),
	rssreader.WithMaxBodySize(5*1024*1024),
)
```

By default:
* **Concurrency limit**: `10` simultaneous requests.
* **Max body size**: `10 MB` per feed.

---

## Handling Partial Failures

When parsing multiple feeds, some might fail (due to DNS, timeouts, or server errors) while others succeed. The parser returns successfully parsed items **alongside** a `GroupedErrors` value.

```go
items, err := reader.Parse(ctx, urls)
if err != nil {
	var ge rssreader.GroupedErrors
	if errors.As(err, &ge) {
		for url, errs := range ge.ByURL() {
			log.Printf("Feed %s failed %d time(s):", url, len(errs))
			for _, ue := range errs {
				log.Printf("  - %v", ue.Unwrap())
			}
		}
	}
}

// items still contains all successfully parsed entries!
```

---

## Timeout / Cancellation

You can enforce limits on execution time or cancel in-flight parsing operations dynamically using the standard library's `context` package:

```go
ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
defer cancel()

items, err := reader.Parse(ctx, urls)
```

---

## API Reference

### `Date`

Represents a calendar date (year, month, day) with no time or timezone component.

```go
type Date struct {
	Year  int
	Month time.Month
	Day   int
}

func DateFromTime(t time.Time) Date  // Converts time.Time → Date (UTC)
func (d Date) Time() time.Time       // Returns midnight UTC for comparisons
func (d Date) String() string        // Returns "YYYY-MM-DD"
func (d Date) IsZero() bool          // Reports if it is the zero Date{}
func (d Date) Before(u Date) bool    // Checks if d is earlier than u
func (d Date) After(u Date) bool     // Checks if d is later than u
func (d Date) Equal(u Date) bool     // Checks if d and u are identical calendar dates
```

### `RssItem`

```go
type RssItem struct {
	Title       string
	Source      string // Feed / channel title
	SourceURL   string // URL the feed was fetched from
	Link        string // URL of the individual post (dedup key)
	PublishDate Date   // Calendar date only — no time or timezone
	Description string
}
```

### `Option`

```go
func WithConcurrencyLimit(limit int) Option  // Defaults to 10. Ignored if limit <= 0
func WithMaxBodySize(size int64) Option      // Defaults to 10MB (10,485,760 bytes)
```

### `RssReader`

```go
func New(opts ...Option) RssReader

type RssReader interface {
	Parse(ctx context.Context, urls []string) ([]RssItem, error)
}
```

### Error Types

```go
// URLError binds a failure to a specific URL
type URLError interface {
	error
	URL() string
	Unwrap() error
}

// ParseError aggregates multiple URLErrors
type ParseError interface {
	error
	Errors() []URLError
	Unwrap() []error // Go 1.20+ multi-unwrap
}

// GroupedErrors maps URLErrors to their originating URL
type GroupedErrors interface {
	ParseError
	ByURL() map[string][]URLError
}
```

---

## Running Tests

All unit tests mock external network requests using `httptest.NewServer` to guarantee fast, offline, and race-free executions:

```bash
go test -v -race ./...
```

To run the live integration tests that fetch actual public RSS and Atom feeds:

```bash
go test -v -tags integration ./...
```

---

## License

This project is licensed under the MIT License.
