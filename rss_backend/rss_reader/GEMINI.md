# RSS Reader Package — Implementation Plan

## Decisions

| Concern | Decision |
|---|---|
| Error handling | Collect errors per URL; return partial results **alongside** a combined error |
| Duplicate items | **Deduplicate by `Link`** (item URL) across all feeds |
| Timeout / context | `Parse` accepts `context.Context` as first parameter |
| Feed formats | **Both RSS 2.0 and Atom** (try RSS first, then Atom — no format detection) |
| Item ordering | **Sort by `PublishDate` descending** (newest first; zero dates sort last) |
| Configuration | **Functional options** — `New(WithConcurrencyLimit(10))` |
| Publish date | **`Date` type** — `Year`, `Month`, `Day` fields; no time or timezone |

---

## Package Layout

```
rssreader/
├── go.mod
├── go.sum
├── rssreader.go        # Public API: Date, RssItem, RssReader, New, options
├── fetch.go            # HTTP fetching
├── parse_rss.go        # RSS 2.0 XML parsing
├── parse_atom.go       # Atom XML parsing
├── errors.go           # URLError (interface), ParseError (interface), GroupedErrors
├── rssreader_test.go   # All tests
└── README.md
```

> `detect.go` has been removed — format detection is replaced by trying RSS
> then Atom in sequence inside `fetchAndParse`.

---

## Public API

### Type: `Date`

```go
type Date struct {
    Year  int
    Month time.Month
    Day   int
}

func DateFromTime(t time.Time) Date  // converts time.Time → Date (UTC)
func (d Date) Time() time.Time       // midnight UTC — used for comparisons
func (d Date) String() string        // "YYYY-MM-DD"
func (d Date) IsZero() bool
func (d Date) Before(u Date) bool
func (d Date) After(u Date) bool
func (d Date) Equal(u Date) bool
```

### Type: `RssItem`

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

### Functional options

```go
func WithConcurrencyLimit(limit int) Option  // default: 10
func WithMaxBodySize(size int64) Option      // default: 10 MB
```

### Constructor & interface

```go
func New(opts ...Option) RssReader

type RssReader interface {
    Parse(ctx context.Context, urls []string) ([]RssItem, error)
}
```

- Spawns one goroutine per URL, gated by a semaphore (default cap: **10**).
- Each goroutine fetches then tries RSS → Atom parsing in sequence.
- Results are merged, **deduplicated by `Link`** (first occurrence wins), then
  **sorted by `PublishDate` descending** (zero dates sort last).
- If ≥1 URL fails, a `GroupedErrors` is returned alongside whatever items succeeded.
- Caller controls timeout via `ctx`:

```go
ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
defer cancel()
items, err := reader.Parse(ctx, urls)
```

---

## Error types

### `URLError` (interface)

```go
type URLError interface {
    error
    URL() string
    Unwrap() error
}
```

### `ParseError` (interface)

```go
type ParseError interface {
    error
    Errors() []URLError
    Unwrap() []error  // Go 1.20+ multi-unwrap
}
```

### `GroupedErrors` (interface, returned by Parse)

```go
type GroupedErrors interface {
    ParseError
    ByURL() map[string][]URLError
}
```

Callers can inspect failures at any level:

```go
items, err := reader.Parse(ctx, urls)
if err != nil {
    var ge rssreader.GroupedErrors
    if errors.As(err, &ge) {
        for url, errs := range ge.ByURL() {
            log.Printf("URL %s failed %d time(s):", url, len(errs))
            for _, ue := range errs {
                log.Printf("  - %v", ue.Unwrap())
            }
        }
    }
}
// items contains all successfully parsed entries
```

---

## Internal Design

### Fetching (`fetch.go`)

- Uses `net/http` with the provided `context.Context`.
- `User-Agent: rssreader/1.0`.
- Body capped at configured `maxBodySize` (default **10 MB**).

### Format detection — removed

`detect.go` is dropped. `fetchAndParse` simply tries RSS then Atom:

```go
if items, err := parseRSS(data, url); err == nil {
    return items, nil
}
if items, err := parseAtom(data, url); err == nil {
    return items, nil
}
return nil, fmt.Errorf("unable to parse feed at %s: unsupported or malformed format", url)
```

### RSS 2.0 Parser (`parse_rss.go`)

```
<channel>
  <title>         → RssItem.Source
  <item>
    <title>       → RssItem.Title
    <link>        → RssItem.Link
    <pubDate>     → DateFromTime(parsed RFC1123/RFC1123Z) → RssItem.PublishDate
    <description> → RssItem.Description
```

### Atom Parser (`parse_atom.go`)

```
<feed>
  <title>              → RssItem.Source
  <entry>
    <title>            → RssItem.Title
    <link href="...">  → RssItem.Link
    <published>        → DateFromTime(parsed RFC3339) → RssItem.PublishDate
                         (fallback: <updated>)
    <summary>          → RssItem.Description (fallback: <content>)
```

### Deduplication (`deduplicate()`)

Extracted to its own unexported function. Uses `map[string]struct{}` (zero
memory per entry). Items with an empty `Link` are always kept.

### Sorting (`sortItems()`)

Extracted to its own unexported function. Zero `Date` values sort to the end:

```go
if di.IsZero() { return false }
if dj.IsZero() { return true  }
return di.After(dj)
```

### Concurrency model

```
New(opts...) → rssReader{cfg}
  │
Parse(ctx, urls)
  │
  ├─ semaphore channel (cap = cfg.concurrencyLimit)
  ├─ sync.WaitGroup
  └─ resultsChan  chan result
        │
        ├─ goroutine per url
        │     check ctx.Done before semaphore
        │     acquire semaphore
        │     fetchAndParse(ctx, url)
        │       └─ fetchURL → try parseRSS → try parseAtom → error
        │     release semaphore
        │     send result to resultsChan
        │
        └─ drain resultsChan
              deduplicate(allItems)
              sortItems(items)
              newParseErrors(urlErrs) → GroupedErrors | nil
              return items, err
```

---

## Test Plan (`rssreader_test.go`)

All tests use `httptest.NewServer` — **no real network calls** unless the
`integration` build tag is set.

### Unit tests

| Test | What it covers |
|---|---|
| `TestParseRSS2_ValidFeed` | RSS 2.0 XML → correct `RssItem` fields including `Date` |
| `TestParseAtom_ValidFeed` | Atom XML → correct `RssItem` fields including `Date` |
| `TestParseMixedFormats` | One RSS + one Atom mock server, results merged |
| `TestParseDeduplication` | Same `Link` in two feeds → only one item returned |
| `TestParsePublishDateFormats` | RFC1123, RFC1123Z, RFC3339, missing date → `Date.IsZero()` |
| `TestParseSortOrder` | Items sorted newest-first; zero dates at the end |
| `TestParseEmptyURLList` | `urls = []string{}` → empty slice, `nil` error |
| `TestParsePartialFailure` | One valid + one 500 server → items + `GroupedErrors` with one entry |
| `TestParseAllURLsFail` | All servers error → empty slice + `GroupedErrors` with all entries |
| `TestParseInvalidXML` | Malformed XML → error collected, other URLs succeed |
| `TestParseContextCancelled` | `ctx` cancelled → returns quickly, errors collected |
| `TestParseContextTimeout` | `context.WithTimeout` of 1ms → all fetches fail gracefully |
| `TestConcurrencyLimit` | 20 URLs; assert ≤10 in-flight via atomic counter |
| `TestGroupedErrors_ByURL` | Same URL fails twice → grouped correctly |
| `TestParseErrors_Unwrap` | `errors.As` works through `Unwrap()` chain |
| `TestDate_String` | `Date.String()` returns `"YYYY-MM-DD"` |
| `TestDate_IsZero` | Zero `Date{}` reports `IsZero() == true` |
| `TestDate_BeforeAfterEqual` | Comparison methods correct for known dates |
| `TestDateFromTime` | UTC conversion strips time component |
| `TestWithConcurrencyLimit` | Option sets cfg correctly; invalid value ignored |
| `TestWithMaxBodySize` | Option sets cfg correctly; invalid value ignored |

### Integration tests (build tag: `integration`)

```go
//go:build integration
```

Fetches 2 real public feeds and asserts non-empty results with non-zero `PublishDate` values.

---

## Dependencies

| Package | Purpose |
|---|---|
| `encoding/xml` (stdlib) | XML parsing |
| `net/http` (stdlib) | HTTP fetching |
| `context` (stdlib) | Cancellation / timeouts |
| `sort` (stdlib) | Sort by `PublishDate` |
| `sync` (stdlib) | `WaitGroup`; semaphore via buffered channel |
| `time` (stdlib) | `time.Month` in `Date`; `DateFromTime` conversion |
| `net/http/httptest` (stdlib) | Mock servers in tests |

No third-party dependencies — zero entries in `go.sum` beyond the module itself.

---

## Go Module

```
module github.com/<username>/rssreader

go 1.23
```

---

## README Outline

1. **Installation** — `go get github.com/<username>/rssreader`
2. **Quick start** — minimal working snippet
3. **Configuration** — `WithConcurrencyLimit`, `WithMaxBodySize`
4. **Handling partial failures** — `errors.As` + `GroupedErrors.ByURL()`
5. **Timeout / cancellation** — `context.WithTimeout` example
6. **API reference** — `New`, `Parse`, `Date`, `RssItem`, `GroupedErrors`, `ParseError`, `URLError`
7. **Running tests** — `go test ./...` and `go test -tags integration ./...`

---

## Implementation Order

1. `go.mod`
2. `errors.go` — `URLError`, `ParseError`, `GroupedErrors` interfaces + unexported impls
3. `rssreader.go` — `Date` type + all methods, `RssItem`, `Option`, `New`, `RssReader` interface stub
4. `fetch.go` — HTTP fetch with context + body cap
5. `parse_rss.go` — RSS 2.0 parser (use `DateFromTime`)
6. `parse_atom.go` — Atom parser (use `DateFromTime`)
7. `rssreader.go` — wire up `Parse`, `fetchAndParse`, `deduplicate`, `sortItems`
8. `rssreader_test.go` — all unit tests green
9. `README.md`
10. `go mod tidy`
