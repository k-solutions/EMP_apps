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
	"regexp"
	"strings"
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

	// Declare and bind command sniff queue for JSON schema contract verification
	cmdSniffQueue := "int_commands_test_sniff"
	_, err = ch.QueueDeclare(
		cmdSniffQueue,
		false, // durable
		true,  // auto-delete
		true,  // exclusive
		false, // no-wait
		nil,
	)
	if err != nil {
		t.Fatalf("failed to declare command sniff queue: %v", err)
	}
	err = ch.QueueBind(
		cmdSniffQueue,
		cfg.RabbitMQ.RoutingKeys.CommandsWorker,
		cfg.RabbitMQ.Exchanges.Commands,
		false,
		nil,
	)
	if err != nil {
		t.Fatalf("failed to bind command sniff queue: %v", err)
	}

	// Declare and bind result sniff queue for JSON schema contract verification
	resultSniffQueue := "int_results_test_sniff"
	_, err = ch.QueueDeclare(
		resultSniffQueue,
		false, // durable
		true,  // auto-delete
		true,  // exclusive
		false, // no-wait
		nil,
	)
	if err != nil {
		t.Fatalf("failed to declare result sniff queue: %v", err)
	}
	err = ch.QueueBind(
		resultSniffQueue,
		cfg.RabbitMQ.RoutingKeys.ResultsRails,
		cfg.RabbitMQ.Exchanges.Results,
		false,
		nil,
	)
	if err != nil {
		t.Fatalf("failed to bind result sniff queue: %v", err)
	}

	cmdDeliveries, err := ch.Consume(
		cmdSniffQueue,
		"cmd-sniffer",
		true,  // auto-ack
		true,  // exclusive
		false, // no-local
		false, // no-wait
		nil,
	)
	if err != nil {
		t.Fatalf("failed to consume command sniff queue: %v", err)
	}

	resultDeliveries, err := ch.Consume(
		resultSniffQueue,
		"result-sniffer",
		true,  // auto-ack
		true,  // exclusive
		false, // no-local
		false, // no-wait
		nil,
	)
	if err != nil {
		t.Fatalf("failed to consume result sniff queue: %v", err)
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

	// 4. Sniff and validate the RabbitMQ JSON payloads against standard Async API specifications
	var capturedCmdBytes []byte
	select {
	case d := <-cmdDeliveries:
		capturedCmdBytes = d.Body
	case <-time.After(5 * time.Second):
		t.Error("timed out waiting for command message to be sniffed")
	}

	var capturedResultBytes []byte
	select {
	case d := <-resultDeliveries:
		capturedResultBytes = d.Body
	case <-time.After(5 * time.Second):
		t.Error("timed out waiting for result message to be sniffed")
	}

	if len(capturedCmdBytes) > 0 {
		validateCommandSchema(t, capturedCmdBytes)
	}
	if len(capturedResultBytes) > 0 {
		validateResultSchema(t, capturedResultBytes)
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

func validateCommandSchema(t *testing.T, data []byte) {
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Command payload is not valid JSON: %v", err)
	}

	// Required fields: job_id, urls
	jobIDVal, exists := raw["job_id"]
	if !exists {
		t.Error("Command payload missing required field 'job_id'")
	}
	jobID, ok := jobIDVal.(string)
	if !ok {
		t.Errorf("Command 'job_id' is not a string, got %T", jobIDVal)
	} else {
		// Verify ULID regex: ^[0-9A-HJKMNP-TV-Z]{26}$
		matched, err := regexp.MatchString("^[0-9A-HJKMNP-TV-Z]{26}$", jobID)
		if err != nil {
			t.Fatalf("regex error: %v", err)
		}
		if !matched {
			t.Errorf("Command 'job_id' %q does not match ULID pattern", jobID)
		}
	}

	urlsVal, exists := raw["urls"]
	if !exists {
		t.Error("Command payload missing required field 'urls'")
	}
	urls, ok := urlsVal.([]interface{})
	if !ok {
		t.Errorf("Command 'urls' is not an array, got %T", urlsVal)
	} else {
		if len(urls) < 1 {
			t.Error("Command 'urls' must contain at least 1 item")
		}
		for i, uVal := range urls {
			u, ok := uVal.(string)
			if !ok {
				t.Errorf("Command 'urls[%d]' is not a string, got %T", i, uVal)
			} else {
				if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
					t.Errorf("Command 'urls[%d]' value %q is not a valid HTTP/HTTPS URI", i, u)
				}
			}
		}
	}
}

func validateResultSchema(t *testing.T, data []byte) {
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Result payload is not valid JSON: %v", err)
	}

	// Required fields: job_id, status
	jobIDVal, exists := raw["job_id"]
	if !exists {
		t.Error("Result payload missing required field 'job_id'")
	}
	jobID, ok := jobIDVal.(string)
	if !ok {
		t.Errorf("Result 'job_id' is not a string, got %T", jobIDVal)
	} else {
		matched, _ := regexp.MatchString("^[0-9A-HJKMNP-TV-Z]{26}$", jobID)
		if !matched {
			t.Errorf("Result 'job_id' %q does not match ULID pattern", jobID)
		}
	}

	statusVal, exists := raw["status"]
	if !exists {
		t.Error("Result payload missing required field 'status'")
	}
	status, ok := statusVal.(string)
	if !ok {
		t.Errorf("Result 'status' is not a string, got %T", statusVal)
	} else {
		if status != "done" && status != "failed" {
			t.Errorf("Result 'status' %q must be either 'done' or 'failed'", status)
		}
	}

	// Optional fields: items, errors
	if itemsVal, exists := raw["items"]; exists {
		if itemsVal != nil {
			items, ok := itemsVal.([]interface{})
			if !ok {
				t.Errorf("Result 'items' is not an array, got %T", itemsVal)
			} else {
				for i, itemVal := range items {
					item, ok := itemVal.(map[string]interface{})
					if !ok {
						t.Errorf("Result 'items[%d]' is not an object, got %T", i, itemVal)
						continue
					}
					// Required fields: title, source, source_url, link, publish_date
					for _, reqField := range []string{"title", "source", "source_url", "link", "publish_date"} {
						if _, exists := item[reqField]; !exists {
							t.Errorf("Result 'items[%d]' missing required field %q", i, reqField)
						}
					}
					if val, exists := item["source_url"]; exists {
						s, ok := val.(string)
						if !ok || (!strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://")) {
							t.Errorf("Result 'items[%d].source_url' must be a valid URI, got %v", i, val)
						}
					}
					if val, exists := item["link"]; exists {
						s, ok := val.(string)
						if !ok || (!strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://")) {
							t.Errorf("Result 'items[%d].link' must be a valid URI, got %v", i, val)
						}
					}
					if val, exists := item["publish_date"]; exists {
						s, ok := val.(string)
						if !ok {
							t.Errorf("Result 'items[%d].publish_date' must be a string, got %T", i, val)
						} else {
							matched, _ := regexp.MatchString(`^\d{4}-\d{2}-\d{2}$`, s)
							if !matched {
								t.Errorf("Result 'items[%d].publish_date' %q does not match YYYY-MM-DD pattern", i, s)
							}
						}
					}
				}
			}
		}
	}

	if errorsVal, exists := raw["errors"]; exists {
		if errorsVal != nil {
			errors, ok := errorsVal.([]interface{})
			if !ok {
				t.Errorf("Result 'errors' is not an array, got %T", errorsVal)
			} else {
				for i, errVal := range errors {
					errItem, ok := errVal.(map[string]interface{})
					if !ok {
						t.Errorf("Result 'errors[%d]' is not an object, got %T", i, errVal)
						continue
					}
					// Required fields: url, error
					for _, reqField := range []string{"url", "error"} {
						if _, exists := errItem[reqField]; !exists {
							t.Errorf("Result 'errors[%d]' missing required field %q", i, reqField)
						}
					}
					if val, exists := errItem["url"]; exists {
						s, ok := val.(string)
						if !ok || (!strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://")) {
							t.Errorf("Result 'errors[%d].url' must be a valid URI, got %v", i, val)
						}
					}
					if val, exists := errItem["error"]; exists {
						if _, ok := val.(string); !ok {
							t.Errorf("Result 'errors[%d].error' must be a string, got %T", i, val)
						}
					}
				}
			}
		}
	}
}
