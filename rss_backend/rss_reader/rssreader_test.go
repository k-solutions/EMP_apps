package rssreader

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const validRSS = `<?xml version="1.0" encoding="utf-8"?>
<rss version="2.0">
  <channel>
    <title>Test RSS Feed</title>
    <link>http://example.com/feed</link>
    <description>This is a test feed</description>
    <item>
      <title>RSS Item 1</title>
      <link>http://example.com/rss1</link>
      <pubDate>Mon, 02 Jan 2006 15:04:05 -0700</pubDate>
      <description>RSS Description 1</description>
    </item>
    <item>
      <title>RSS Item 2</title>
      <link>http://example.com/rss2</link>
      <pubDate>Mon, 02 Jan 2006 15:04:05 MST</pubDate>
      <description>RSS Description 2</description>
    </item>
  </channel>
</rss>`

const validAtom = `<?xml version="1.0" encoding="utf-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Test Atom Feed</title>
  <link href="http://example.com/feed" />
  <entry>
    <title>Atom Entry 1</title>
    <link href="http://example.com/atom1" />
    <published>2006-01-02T15:04:05Z</published>
    <summary>Atom Summary 1</summary>
  </entry>
  <entry>
    <title>Atom Entry 2</title>
    <link rel="alternate" href="http://example.com/atom2" />
    <updated>2006-01-02T15:04:05-07:00</updated>
    <content>Atom Content 2</content>
  </entry>
</feed>`

func TestParseRSS2_ValidFeed(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(validRSS))
	}))
	defer ts.Close()

	items, err := New().Parse(context.Background(), []string{ts.URL})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	// Verify fields
	for _, item := range items {
		if item.Source != "Test RSS Feed" {
			t.Errorf("expected Source 'Test RSS Feed', got: %s", item.Source)
		}
		if item.SourceURL != ts.URL {
			t.Errorf("expected SourceURL '%s', got: %s", ts.URL, item.SourceURL)
		}
		expectedDate := Date{Year: 2006, Month: time.January, Day: 2}
		if item.PublishDate != expectedDate {
			t.Errorf("expected PublishDate %s, got %s", expectedDate, item.PublishDate)
		}
	}
}

func TestParseAtom_ValidFeed(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(validAtom))
	}))
	defer ts.Close()

	items, err := New().Parse(context.Background(), []string{ts.URL})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	var entry1 RssItem
	for _, it := range items {
		if it.Title == "Atom Entry 1" {
			entry1 = it
		}
	}

	if entry1.Title != "Atom Entry 1" {
		t.Fatalf("Atom Entry 1 not found")
	}
	if entry1.Link != "http://example.com/atom1" {
		t.Errorf("expected link http://example.com/atom1, got: %s", entry1.Link)
	}
	if entry1.Source != "Test Atom Feed" {
		t.Errorf("expected Source 'Test Atom Feed', got: %s", entry1.Source)
	}
	expectedDate := Date{Year: 2006, Month: time.January, Day: 2}
	if entry1.PublishDate != expectedDate {
		t.Errorf("expected publish date %v, got: %v", expectedDate, entry1.PublishDate)
	}
	if entry1.Description != "Atom Summary 1" {
		t.Errorf("expected description 'Atom Summary 1', got: %s", entry1.Description)
	}
}

func TestParseMixedFormats(t *testing.T) {
	tsRSS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(validRSS))
	}))
	defer tsRSS.Close()

	tsAtom := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(validAtom))
	}))
	defer tsAtom.Close()

	items, err := New().Parse(context.Background(), []string{tsRSS.URL, tsAtom.URL})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(items) != 4 {
		t.Fatalf("expected 4 items combined, got %d", len(items))
	}
}

func TestParseDeduplication(t *testing.T) {
	duplicateRSS := `<?xml version="1.0" encoding="utf-8"?>
<rss version="2.0">
  <channel>
    <title>Dup Feed</title>
    <item>
      <title>Item A</title>
      <link>http://example.com/dup</link>
    </item>
    <item>
      <title>Item B</title>
      <link>http://example.com/dup</link>
    </item>
    <item>
      <title>Item C</title>
      <link></link>
    </item>
    <item>
      <title>Item D</title>
      <link></link>
    </item>
  </channel>
</rss>`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(duplicateRSS))
	}))
	defer ts.Close()

	items, err := New().Parse(context.Background(), []string{ts.URL})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(items) != 3 {
		t.Fatalf("expected 3 items after deduplication, got %d: %+v", len(items), items)
	}
}

func TestParsePublishDateFormats(t *testing.T) {
	testFeed := `<?xml version="1.0" encoding="utf-8"?>
<rss version="2.0">
  <channel>
    <title>Date Formats</title>
    <item>
      <title>RFC1123Z</title>
      <pubDate>Mon, 02 Jan 2006 15:04:05 -0700</pubDate>
    </item>
    <item>
      <title>RFC1123</title>
      <pubDate>Mon, 02 Jan 2006 15:04:05 MST</pubDate>
    </item>
    <item>
      <title>Fallback MST space</title>
      <pubDate>Mon,  2 Jan 2006 15:04:05 MST</pubDate>
    </item>
    <item>
      <title>Empty Date</title>
      <pubDate></pubDate>
    </item>
  </channel>
</rss>`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(testFeed))
	}))
	defer ts.Close()

	items, err := New().Parse(context.Background(), []string{ts.URL})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(items) != 4 {
		t.Fatalf("expected 4 items, got %d", len(items))
	}

	for _, item := range items {
		if item.Title == "Empty Date" {
			if !item.PublishDate.IsZero() {
				t.Errorf("expected zero date for Empty Date, got %v", item.PublishDate)
			}
		} else {
			if item.PublishDate.IsZero() {
				t.Errorf("expected non-zero date for %s", item.Title)
			}
		}
	}
}

func TestParseSortOrder(t *testing.T) {
	feed := `<?xml version="1.0" encoding="utf-8"?>
<rss version="2.0">
  <channel>
    <title>Sort Feed</title>
    <item>
      <title>Oldest</title>
      <pubDate>Mon, 02 Jan 2006 15:04:05 -0700</pubDate>
    </item>
    <item>
      <title>Newest</title>
      <pubDate>Tue, 03 Jan 2006 15:04:05 -0700</pubDate>
    </item>
    <item>
      <title>Middle</title>
      <pubDate>Mon, 02 Jan 2006 20:04:05 -0700</pubDate>
    </item>
    <item>
      <title>ZeroDate</title>
      <pubDate></pubDate>
    </item>
  </channel>
</rss>`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(feed))
	}))
	defer ts.Close()

	items, err := New().Parse(context.Background(), []string{ts.URL})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(items) != 4 {
		t.Fatalf("expected 4 items, got %d", len(items))
	}

	// Should be sorted Newest (Jan 3), Middle (Jan 2), Oldest (Jan 2), then ZeroDate at the end.
	// Note: Middle and Oldest are both on Jan 2 calendar-wise because time components are stripped.
	// Since our Date represents calendar date, they will both be Date{2006, 1, 2}.
	if items[0].Title != "Newest" {
		t.Errorf("expected first item to be 'Newest', got '%s'", items[0].Title)
	}
	if items[3].Title != "ZeroDate" {
		t.Errorf("expected last item to be 'ZeroDate', got '%s'", items[3].Title)
	}
}

func TestParseEmptyURLList(t *testing.T) {
	items, err := New().Parse(context.Background(), []string{})
	if err != nil {
		t.Fatalf("expected no error for empty list, got: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}
}

func TestParsePartialFailure(t *testing.T) {
	tsGood := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(validRSS))
	}))
	defer tsGood.Close()

	tsBad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer tsBad.Close()

	items, err := New().Parse(context.Background(), []string{tsGood.URL, tsBad.URL})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var parseErrs ParseError
	if !errors.As(err, &parseErrs) {
		t.Fatalf("expected error of type ParseError, got: %T (%v)", err, err)
	}

	errs := parseErrs.Errors()
	if len(errs) != 1 {
		t.Fatalf("expected 1 error inside ParseErrors, got %d", len(errs))
	}

	if errs[0].URL() != tsBad.URL {
		t.Errorf("expected error URL to be %s, got %s", tsBad.URL, errs[0].URL())
	}

	if len(items) != 2 {
		t.Errorf("expected 2 items from the good feed, got %d", len(items))
	}
}

func TestParseAllURLsFail(t *testing.T) {
	tsBad1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer tsBad1.Close()

	tsBad2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer tsBad2.Close()

	items, err := New().Parse(context.Background(), []string{tsBad1.URL, tsBad2.URL})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var parseErrs ParseError
	if !errors.As(err, &parseErrs) {
		t.Fatalf("expected ParseError, got %T", err)
	}

	if len(parseErrs.Errors()) != 2 {
		t.Errorf("expected 2 errors, got %d", len(parseErrs.Errors()))
	}

	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}
}

func TestParseInvalidXML(t *testing.T) {
	tsGood := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(validRSS))
	}))
	defer tsGood.Close()

	tsBad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("<invalid xml>not even nested</invalid>"))
	}))
	defer tsBad.Close()

	items, err := New().Parse(context.Background(), []string{tsGood.URL, tsBad.URL})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var parseErrs ParseError
	if !errors.As(err, &parseErrs) {
		t.Fatalf("expected ParseError, got %T", err)
	}

	if len(parseErrs.Errors()) != 1 {
		t.Fatalf("expected 1 error, got %d", len(parseErrs.Errors()))
	}

	if len(items) != 2 {
		t.Errorf("expected 2 items, got %d", len(items))
	}
}

func TestParseContextCancelled(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.Write([]byte(validRSS))
	}))
	defer ts.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	items, err := New().Parse(ctx, []string{ts.URL})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}
}

func TestParseContextTimeout(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.Write([]byte(validRSS))
	}))
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	time.Sleep(2 * time.Millisecond)

	_, err := New().Parse(ctx, []string{ts.URL})
	if err == nil {
		t.Fatal("expected error due to timeout, got nil")
	}
}

func TestConcurrencyLimit(t *testing.T) {
	var inFlight int32
	var maxInFlight int32

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		curr := atomic.AddInt32(&inFlight, 1)
		for {
			m := atomic.LoadInt32(&maxInFlight)
			if curr <= m {
				break
			}
			if atomic.CompareAndSwapInt32(&maxInFlight, m, curr) {
				break
			}
		}

		time.Sleep(15 * time.Millisecond)
		atomic.AddInt32(&inFlight, -1)

		w.Write([]byte(validRSS))
	}))
	defer ts.Close()

	urls := make([]string, 20)
	for i := range urls {
		urls[i] = ts.URL
	}

	_, err := New().Parse(context.Background(), urls)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	maxVal := atomic.LoadInt32(&maxInFlight)
	if maxVal > 10 {
		t.Errorf("expected max concurrent requests <= 10, got %d", maxVal)
	}
}

func TestParseErrors_Error(t *testing.T) {
	errs := []URLError{
		newURLError("http://err1", errors.New("err1")),
		newURLError("http://err2", errors.New("err2")),
	}
	pe := newParseErrors(errs)

	errStr := pe.Error()

	expectedPrefix := "rssreader parse errors:"
	if len(errStr) < len(expectedPrefix) || errStr[:len(expectedPrefix)] != expectedPrefix {
		t.Errorf("unexpected error string: %s", errStr)
	}
}

func TestParseErrors_Unwrap(t *testing.T) {
	targetErr := errors.New("sentinel")
	pe := newParseErrors([]URLError{
		newURLError("http://err", targetErr),
	})

	if !errors.Is(pe, targetErr) {
		t.Errorf("expected errors.Is to find targetErr in ParseErrors")
	}
}

func TestGroupedErrors_ByURL(t *testing.T) {
	err1 := newURLError("http://err1", errors.New("err1"))
	err2 := newURLError("http://err1", errors.New("err1-2"))
	err3 := newURLError("http://err2", errors.New("err2"))

	pe := newParseErrors([]URLError{err1, err2, err3})

	var ge GroupedErrors
	if !errors.As(pe, &ge) {
		t.Fatalf("expected errors.As to find GroupedErrors interface")
	}

	byURL := ge.ByURL()
	if len(byURL) != 2 {
		t.Errorf("expected 2 URLs in grouped errors, got %d", len(byURL))
	}

	if len(byURL["http://err1"]) != 2 {
		t.Errorf("expected 2 errors for http://err1, got %d", len(byURL["http://err1"]))
	}

	if len(byURL["http://err2"]) != 1 {
		t.Errorf("expected 1 error for http://err2, got %d", len(byURL["http://err2"]))
	}
}

func TestDate_String(t *testing.T) {
	d := Date{Year: 2026, Month: time.May, Day: 23}
	if d.String() != "2026-05-23" {
		t.Errorf("expected String() to return \"2026-05-23\", got %q", d.String())
	}
}

func TestDate_IsZero(t *testing.T) {
	var d Date
	if !d.IsZero() {
		t.Error("expected default Date{} to report IsZero == true")
	}
	d2 := Date{Year: 2026, Month: time.May, Day: 23}
	if d2.IsZero() {
		t.Error("expected non-zero date to report IsZero == false")
	}
}

func TestDate_BeforeAfterEqual(t *testing.T) {
	d1 := Date{Year: 2026, Month: time.May, Day: 23}
	d2 := Date{Year: 2026, Month: time.May, Day: 24}
	if !d1.Before(d2) {
		t.Errorf("expected %s Before %s", d1, d2)
	}
	if d2.Before(d1) {
		t.Errorf("expected %s NOT Before %s", d2, d1)
	}
	if !d2.After(d1) {
		t.Errorf("expected %s After %s", d2, d1)
	}
	if d1.After(d2) {
		t.Errorf("expected %s NOT After %s", d1, d2)
	}
	if !d1.Equal(Date{Year: 2026, Month: time.May, Day: 23}) {
		t.Errorf("expected %s Equal to identical date", d1)
	}
}

func TestDateFromTime(t *testing.T) {
	tVal := time.Date(2026, time.May, 23, 15, 30, 45, 0, time.FixedZone("EDT", -4*3600))
	d := DateFromTime(tVal)
	expectedUTC := tVal.UTC()
	expectedD := Date{Year: expectedUTC.Year(), Month: expectedUTC.Month(), Day: expectedUTC.Day()}
	if d != expectedD {
		t.Errorf("expected DateFromTime to strip timezone and use UTC: expected %s, got %s", expectedD, d)
	}
}

func TestWithConcurrencyLimit(t *testing.T) {
	cfg := &config{}
	WithConcurrencyLimit(5)(cfg)
	if cfg.concurrencyLimit != 5 {
		t.Errorf("expected concurrency limit to be 5, got %d", cfg.concurrencyLimit)
	}
	WithConcurrencyLimit(-1)(cfg)
	if cfg.concurrencyLimit != 5 {
		t.Errorf("expected invalid concurrency limit to be ignored, got %d", cfg.concurrencyLimit)
	}
}

func TestWithMaxBodySize(t *testing.T) {
	cfg := &config{}
	WithMaxBodySize(100)(cfg)
	if cfg.maxBodySize != 100 {
		t.Errorf("expected max body size to be 100, got %d", cfg.maxBodySize)
	}
	WithMaxBodySize(-10)(cfg)
	if cfg.maxBodySize != 100 {
		t.Errorf("expected invalid max body size to be ignored, got %d", cfg.maxBodySize)
	}
}

func TestOptionsConfiguration(t *testing.T) {
	// 1. Verify custom MaxBodySize via option
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("this response body is definitely larger than fifty bytes in length"))
	}))
	defer ts.Close()

	reader := New(WithMaxBodySize(50))
	_, err := reader.Parse(context.Background(), []string{ts.URL})
	if err == nil {
		t.Fatal("expected error due to response body size limit, got nil")
	}
	if !strings.Contains(err.Error(), "exceeds maximum size") {
		t.Errorf("expected body size limit error, got: %v", err)
	}

	// 2. Verify custom ConcurrencyLimit via option
	var inFlight int32
	var maxInFlight int32
	tsLimit := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		curr := atomic.AddInt32(&inFlight, 1)
		for {
			m := atomic.LoadInt32(&maxInFlight)
			if curr <= m {
				break
			}
			if atomic.CompareAndSwapInt32(&maxInFlight, m, curr) {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
		atomic.AddInt32(&inFlight, -1)
		w.Write([]byte(validRSS))
	}))
	defer tsLimit.Close()

	urls := make([]string, 10)
	for i := range urls {
		urls[i] = tsLimit.URL
	}

	readerLimit := New(WithConcurrencyLimit(3))
	_, err = readerLimit.Parse(context.Background(), urls)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	maxVal := atomic.LoadInt32(&maxInFlight)
	if maxVal > 3 {
		t.Errorf("expected max concurrent requests <= 3, got %d", maxVal)
	}
}
