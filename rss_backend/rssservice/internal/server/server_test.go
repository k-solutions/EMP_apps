package server

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/emarchant/rssreader"
	"github.com/emarchant/rssservice/internal/auth"
	"github.com/emarchant/rssservice/internal/jobstore"
)

type mockRssReader struct{}

func (m *mockRssReader) Parse(ctx context.Context, urls []string) ([]rssreader.RssItem, error) {
	return nil, nil
}

func TestServerRouting(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	store := jobstore.NewInMemoryJobStore()
	reader := &mockRssReader{}

	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	validator := auth.NewJWTValidatorFromKey(&privKey.PublicKey, nil, "test:")

	srv := NewServer(
		8080,
		5*time.Second, 5*time.Second, 5*time.Second,
		reader,
		store,
		nil,
		nil,
		validator,
		true,
		logger,
	)

	// We can test the handler of srv.httpServer directly by serving HTTP requests
	mux := srv.httpServer.Handler

	// 1. Health check should return 200
	req1 := httptest.NewRequest("GET", "/health", nil)
	w1 := httptest.NewRecorder()
	mux.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Errorf("expected 200 health, got %d", w1.Code)
	}

	// 2. Swagger docs should return 200
	req2 := httptest.NewRequest("GET", "/docs", nil)
	w2 := httptest.NewRecorder()
	mux.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Errorf("expected 200 docs, got %d", w2.Code)
	}

	// 3. Authenticated route without JWT should return 401
	req3 := httptest.NewRequest("POST", "/parse", nil)
	w3 := httptest.NewRecorder()
	mux.ServeHTTP(w3, req3)
	if w3.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 unauthorized, got %d", w3.Code)
	}
}
