//go:build integration

package server

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/emarchant/rssreader"
	"github.com/emarchant/rssservice/internal/auth"
	"github.com/emarchant/rssservice/internal/cache"
	"github.com/emarchant/rssservice/internal/config"
	"github.com/emarchant/rssservice/internal/jobstore"
	"github.com/emarchant/rssservice/internal/rabbitmq"
	"github.com/emarchant/rssservice/internal/server/handler"
	"github.com/emarchant/rssservice/internal/worker"
	"github.com/golang-jwt/jwt/v5"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"
)

type mockIntegrationReader struct{}

func (m *mockIntegrationReader) Parse(ctx context.Context, urls []string) ([]rssreader.RssItem, error) {
	return []rssreader.RssItem{
		{
			Title:       "Integration Test Post",
			Source:      "Integration Source",
			SourceURL:   urls[0],
			Link:        "https://example.com/integration-item",
			PublishDate: rssreader.Date{Year: 2026, Month: time.May, Day: 24},
			Description: "Body",
		},
	}, nil
}

func TestIntegrationRoundTrip(t *testing.T) {
	// 1. Setup Redis Guard
	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Skip("skipping integration round trip test: local Redis not running")
	}
	defer rdb.Close()

	// 2. Setup RabbitMQ Guard
	rabbitURL := "amqp://guest:guest@localhost:5672/"
	testConn, err := amqp.Dial(rabbitURL)
	if err != nil {
		t.Skip("skipping integration round trip test: local RabbitMQ not running")
	}
	testConn.Close()

	// 3. Isolated configuration structures
	cfg := &config.Config{
		Server: config.ServerConfig{
			Port: 8080,
		},
		Redis: config.RedisConfig{
			DSN:             "redis://localhost:6379/0",
			JobTTL:          config.Duration(10 * time.Minute),
			CacheTTLSeconds: 60,
			BlocklistPrefix: "int:blocklist:",
		},
		RabbitMQ: config.RabbitMQConfig{
			URL: rabbitURL,
			Exchanges: config.ExchangesConfig{
				Commands: "int_commands",
				Results:  "int_results",
				Retry:    "int_retry",
			},
			Queues: config.QueuesConfig{
				Worker:  "int_commands_worker",
				Wait30s: "int_commands_wait_30s",
				Failed:  "int_commands_failed",
			},
			RoutingKeys: config.RoutingKeysConfig{
				CommandsWorker: "int_commands_worker",
				ResultsRails:   "int_results_rails",
			},
			ConsumerTag: "int-go-worker-1",
			Prefetch:    10,
		},
	}

	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))

	// Cleanup Redis database keys
	rdb.FlushDB(ctx)

	// Setup Keys and Auth
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate EC key: %v", err)
	}
	validator := auth.NewJWTValidatorFromKey(&privKey.PublicKey, rdb, cfg.Redis.BlocklistPrefix)

	claims := &auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "int-test-user",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
		},
	}
	tokenStr, _ := jwt.NewWithClaims(jwt.SigningMethodES256, claims).SignedString(privKey)

	reader := &mockIntegrationReader{}
	store := jobstore.NewRedisJobStore(rdb, time.Duration(cfg.Redis.JobTTL))
	urlCache := cache.NewURLCache(rdb, time.Duration(cfg.Redis.CacheTTLSeconds)*time.Second)

	// Setup RabbitMQ Client
	client := rabbitmq.NewClient(cfg.RabbitMQ.URL, cfg.RabbitMQ.Prefetch, logger)
	if err := client.Connect(ctx); err != nil {
		t.Fatalf("failed to connect RabbitMQ: %v", err)
	}
	defer client.Close()

	ch, err := client.Channel()
	if err != nil {
		t.Fatalf("failed to get AMQP channel: %v", err)
	}

	if err := rabbitmq.DeclareTopology(ch, cfg); err != nil {
		t.Fatalf("failed to declare topology: %v", err)
	}

	publisher := rabbitmq.NewPublisher(client, cfg)
	consumer := rabbitmq.NewConsumer(client, cfg, logger)
	processor := worker.NewProcessor(reader, store, rdb, urlCache, publisher, logger)

	bgWorker := worker.NewWorker(client, consumer, processor, cfg, logger)
	workerCtx, workerCancel := context.WithCancel(context.Background())
	defer workerCancel()

	go func() {
		_ = bgWorker.Start(workerCtx)
	}()

	// Setup Server
	srv := NewServer(
		cfg.Server.Port,
		5*time.Second, 5*time.Second, 5*time.Second,
		reader, store, publisher, urlCache, validator, false, logger,
	)
	httpHandler := srv.httpServer.Handler

	// 1. Post Parse Request (Async Ingestion Path)
	reqBody := `{"urls": ["https://example.com/rss"]}`
	req1 := httptest.NewRequest("POST", "/parse", bytes.NewReader([]byte(reqBody)))
	req1.Header.Set("Authorization", "Bearer "+tokenStr)
	w1 := httptest.NewRecorder()

	httpHandler.ServeHTTP(w1, req1)

	if w1.Code != http.StatusAccepted {
		t.Fatalf("expected status 202 Accepted, got %d. Body: %s", w1.Code, w1.Body.String())
	}

	var postResp struct {
		JobID string `json:"job_id"`
	}
	_ = json.NewDecoder(w1.Body).Decode(&postResp)
	if postResp.JobID == "" {
		t.Fatal("expected non-empty job ID")
	}

	// 2. Poll Job Status in Redis JobStore
	var finalJob *jobstore.Job
	for i := 0; i < 40; i++ {
		req2 := httptest.NewRequest("GET", "/jobs/"+postResp.JobID, nil)
		req2.Header.Set("Authorization", "Bearer "+tokenStr)
		w2 := httptest.NewRecorder()

		httpHandler.ServeHTTP(w2, req2)
		if w2.Code == http.StatusOK {
			var job jobstore.Job
			_ = json.NewDecoder(w2.Body).Decode(&job)
			if job.Status == "done" {
				finalJob = &job
				break
			}
		}
		time.Sleep(150 * time.Millisecond)
	}

	if finalJob == nil {
		t.Fatal("timed out waiting for job to complete")
	}

	if len(finalJob.Items) != 1 || finalJob.Items[0].Title != "Integration Test Post" {
		t.Errorf("unexpected items details: %+v", finalJob)
	}

	// 3. Verify Redis cache got hydrated
	cachedItems, err := urlCache.Get(ctx, "https://example.com/rss")
	if err != nil {
		t.Errorf("expected URL cache hit, got error: %v", err)
	}
	if len(cachedItems) != 1 || cachedItems[0].Title != "Integration Test Post" {
		t.Errorf("expected cached item to match, got: %+v", cachedItems)
	}
}

func TestIntegrationFallbackRoundTrip(t *testing.T) {
	// Redis Guard
	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Skip("skipping integration fallback test: local Redis not running")
	}
	defer rdb.Close()

	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	validator := auth.NewJWTValidatorFromKey(&privKey.PublicKey, rdb, "int:")

	claims := &auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "int-test-user",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
		},
	}
	tokenStr, _ := jwt.NewWithClaims(jwt.SigningMethodES256, claims).SignedString(privKey)

	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	reader := &mockIntegrationReader{}
	store := jobstore.NewRedisJobStore(rdb, 10*time.Minute)
	urlCache := cache.NewURLCache(rdb, 60*time.Second)

	// Clean database
	rdb.FlushDB(ctx)

	// Setup Server in dynamic fallback (RabbitMQ Publisher is closed/nil, simulating broker down)
	srv := NewServer(
		8080,
		5*time.Second, 5*time.Second, 5*time.Second,
		reader,
		store,
		nil, // nil publisher triggers dynamic synchronous fallback
		urlCache,
		validator,
		false,
		logger,
	)
	httpHandler := srv.httpServer.Handler

	reqBody := `{"urls": ["https://example.com/rss"]}`
	req := httptest.NewRequest("POST", "/parse", bytes.NewReader([]byte(reqBody)))
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	w := httptest.NewRecorder()

	httpHandler.ServeHTTP(w, req)

	// Expecting immediate synchronous success (200 OK)
	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK for fallback, got %d", w.Code)
	}

	var resp handler.FallbackResponse
	_ = json.NewDecoder(w.Body).Decode(&resp)

	if resp.Status != "done" || len(resp.Items) != 1 || resp.Items[0].Title != "Integration Test Post" {
		t.Errorf("unexpected response details in fallback: %+v", resp)
	}

	// Verify that the job was still successfully stored in Redis!
	storedJob, err := store.Get(ctx, resp.JobID)
	if err != nil {
		t.Fatalf("failed to retrieve fallback job from store: %v", err)
	}
	if storedJob.Status != "done" || len(storedJob.Items) != 1 {
		t.Errorf("stored job details mismatched: %+v", storedJob)
	}
}
