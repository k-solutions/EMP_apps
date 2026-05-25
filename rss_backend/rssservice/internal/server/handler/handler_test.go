package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/emarchant/rssreader"
	"github.com/emarchant/rssservice/internal/jobstore"
	"github.com/go-chi/chi/v5"
)

type mockRssReader struct {
	items []rssreader.RssItem
	err   error
}

func (m *mockRssReader) Parse(ctx context.Context, urls []string) ([]rssreader.RssItem, error) {
	return m.items, m.err
}

func TestHealthHandler(t *testing.T) {
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	handler := Health("full")
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp HealthResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	if resp.Status != "ok" || resp.Mode != "full" {
		t.Errorf("unexpected response: %+v", resp)
	}
}

func TestParseHandlerFallback(t *testing.T) {
	// Fallback mode (Redis offline)
	date := rssreader.Date{Year: 2026, Month: time.May, Day: 24}
	mockItems := []rssreader.RssItem{
		{
			Title:       "Fallback Article",
			Source:      "Fallback Feed",
			SourceURL:   "https://example.com/feed",
			Link:        "https://example.com/article",
			PublishDate: date,
		},
	}
	reader := &mockRssReader{items: mockItems, err: nil}
	store := jobstore.NewInMemoryJobStore()

	handler := Parse(reader, store, nil, nil, true)

	// Valid Request
	reqBody := `{"urls": ["https://example.com/feed"]}`
	req := httptest.NewRequest("POST", "/parse", strings.NewReader(reqBody))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp FallbackResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	if resp.Status != "done" || len(resp.Items) != 1 || resp.Items[0].Title != "Fallback Article" {
		t.Errorf("unexpected fallback response: %+v", resp)
	}

	// Verify job was stored
	job, err := store.Get(context.Background(), resp.JobID)
	if err != nil {
		t.Fatalf("expected job to be saved in store: %v", err)
	}
	if job.Status != "done" {
		t.Errorf("expected job status done, got %s", job.Status)
	}
}

func TestJobsHandler(t *testing.T) {
	store := jobstore.NewInMemoryJobStore()
	ctx := context.Background()

	job := &jobstore.Job{
		ID:     "job-123",
		Status: "done",
		Items:  []jobstore.RssItem{{Title: "Test Article", Link: "https://link"}},
	}
	_ = store.Create(ctx, job)

	handler := Jobs(store)

	// Setup a chi context since the handler uses chi.URLParam
	r := chi.NewRouter()
	r.Get("/jobs/{id}", handler)

	// 1. Get existing job
	req1 := httptest.NewRequest("GET", "/jobs/job-123", nil)
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)

	if w1.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w1.Code)
	}

	var retrieved jobstore.Job
	if err := json.NewDecoder(w1.Body).Decode(&retrieved); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if retrieved.ID != "job-123" || len(retrieved.Items) != 1 {
		t.Errorf("retrieved job mismatch: %+v", retrieved)
	}

	// 2. Get missing job
	req2 := httptest.NewRequest("GET", "/jobs/missing-job", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w2.Code)
	}
}
