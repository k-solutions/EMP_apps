package config

import (
	"os"
	"testing"
	"time"
)

func TestLoadConfig(t *testing.T) {
	// Create a temp config file matching the RabbitMQ and caching schema
	content := `
server:
  port: 8080
  read_timeout:  15s
  write_timeout: 30s
  idle_timeout:  60s

redis:
  dsn: "redis://localhost:6379/0"
  job_ttl: 1h
  cache_ttl_seconds: 3600
  blocklist_prefix: "jwt:blocklist:"

rabbitmq:
  url: "amqp://guest:guest@localhost:5672/"
  exchanges:
    commands: "rss_commands"
    results: "rss_results"
    retry: "rss_commands_retry"
  queues:
    worker: "rss_commands_worker"
    wait_30s: "rss_commands_wait_30s"
    failed: "rss_commands_failed"
  routing_keys:
    commands_worker: "rss_commands_worker"
    results_rails: "rss_results_rails"
  consumer_tag: "rss-go-worker-1"
  prefetch: 10

jwt:
  public_key_path: "./keys/ec_public.pem"

rssreader:
  concurrency_limit: 10
  max_body_size:     10485760

log:
  level: "info"
`
	tmpfile, err := os.CreateTemp("", "config_*.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(content)); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatalf("failed to close temp file: %v", err)
	}

	// 1. Test standard loading
	cfg, err := LoadConfig(tmpfile.Name())
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.Server.Port != 8080 {
		t.Errorf("expected Server.Port = 8080, got %d", cfg.Server.Port)
	}
	if time.Duration(cfg.Server.ReadTimeout) != 15*time.Second {
		t.Errorf("expected ReadTimeout = 15s, got %v", cfg.Server.ReadTimeout)
	}
	if cfg.Redis.DSN != "redis://localhost:6379/0" {
		t.Errorf("expected Redis DSN = redis://localhost:6379/0, got %s", cfg.Redis.DSN)
	}
	if time.Duration(cfg.Redis.JobTTL) != time.Hour {
		t.Errorf("expected JobTTL = 1h, got %v", cfg.Redis.JobTTL)
	}
	if cfg.Redis.CacheTTLSeconds != 3600 {
		t.Errorf("expected CacheTTLSeconds = 3600, got %d", cfg.Redis.CacheTTLSeconds)
	}
	if cfg.RabbitMQ.Exchanges.Commands != "rss_commands" {
		t.Errorf("expected exchanges.commands = rss_commands, got %s", cfg.RabbitMQ.Exchanges.Commands)
	}

	// 2. Test environment overrides
	os.Setenv("SERVER_PORT", "9090")
	os.Setenv("SERVER_READ_TIMEOUT", "5s")
	os.Setenv("REDIS_DSN", "redis://localhost:6380/1")
	os.Setenv("REDIS_CACHE_TTL_SECONDS", "1800")
	os.Setenv("RABBITMQ_URL", "amqp://user:pass@localhost:5673/")
	os.Setenv("LOG_LEVEL", "debug")
	defer func() {
		os.Unsetenv("SERVER_PORT")
		os.Unsetenv("SERVER_READ_TIMEOUT")
		os.Unsetenv("REDIS_DSN")
		os.Unsetenv("REDIS_CACHE_TTL_SECONDS")
		os.Unsetenv("RABBITMQ_URL")
		os.Unsetenv("LOG_LEVEL")
	}()

	cfgOverridden, err := LoadConfig(tmpfile.Name())
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfgOverridden.Server.Port != 9090 {
		t.Errorf("expected Server.Port = 9090, got %d", cfgOverridden.Server.Port)
	}
	if time.Duration(cfgOverridden.Server.ReadTimeout) != 5*time.Second {
		t.Errorf("expected ReadTimeout = 5s, got %v", cfgOverridden.Server.ReadTimeout)
	}
	if cfgOverridden.Redis.DSN != "redis://localhost:6380/1" {
		t.Errorf("expected Redis DSN = redis://localhost:6380/1, got %s", cfgOverridden.Redis.DSN)
	}
	if cfgOverridden.Redis.CacheTTLSeconds != 1800 {
		t.Errorf("expected CacheTTLSeconds = 1800, got %d", cfgOverridden.Redis.CacheTTLSeconds)
	}
	if cfgOverridden.RabbitMQ.URL != "amqp://user:pass@localhost:5673/" {
		t.Errorf("expected RabbitMQ.URL override, got %s", cfgOverridden.RabbitMQ.URL)
	}
	if cfgOverridden.Log.Level != "debug" {
		t.Errorf("expected Log.Level = debug, got %s", cfgOverridden.Log.Level)
	}
}
