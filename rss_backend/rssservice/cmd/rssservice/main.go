package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/emarchant/rssreader"
	"github.com/emarchant/rssservice/internal/auth"
	"github.com/emarchant/rssservice/internal/cache"
	"github.com/emarchant/rssservice/internal/config"
	"github.com/emarchant/rssservice/internal/jobstore"
	"github.com/emarchant/rssservice/internal/rabbitmq"
	"github.com/emarchant/rssservice/internal/server"
	"github.com/emarchant/rssservice/internal/worker"
	"github.com/redis/go-redis/v9"
)

func main() {
	// 1. Load config
	configPath := "config.yaml"
	if envPath := os.Getenv("CONFIG_PATH"); envPath != "" {
		configPath = envPath
	}

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		slog.Error("failed to load configuration", "err", err)
		os.Exit(1)
	}

	// 2. Configure Logger
	var programLevel slog.Level
	switch strings.ToLower(cfg.Log.Level) {
	case "debug":
		programLevel = slog.LevelDebug
	case "warn":
		programLevel = slog.LevelWarn
	case "error":
		programLevel = slog.LevelError
	default:
		programLevel = slog.LevelInfo
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: programLevel}))
	slog.SetDefault(logger)

	logger.Info("starting RSS Reader Service")

	// 3. Connect and Ping Redis (dynamic mode detection)
	opt, err := redis.ParseURL(cfg.Redis.DSN)
	var rdb *redis.Client
	isFallback := false

	if err != nil {
		logger.Warn("invalid Redis DSN, starting in fallback mode", "dsn", cfg.Redis.DSN, "err", err)
		isFallback = true
	} else {
		rdb = redis.NewClient(opt)
		pingCtx, pingCancel := context.WithTimeout(context.Background(), 2*time.Second)
		if err := rdb.Ping(pingCtx).Err(); err != nil {
			logger.Warn("Redis is unreachable, starting in fallback mode", "err", err)
			isFallback = true
			rdb.Close()
			rdb = nil
		}
		pingCancel()
	}

	// 4. Setup JobStore & Cache
	var store jobstore.JobStore
	var urlCache *cache.URLCache
	if isFallback {
		logger.Info("operating in fallback mode (in-memory job store and synchronous parsing)")
		store = jobstore.NewInMemoryJobStore()
	} else {
		logger.Info("operating in full mode (Redis job store and asynchronous RabbitMQ workers)")
		store = jobstore.NewRedisJobStore(rdb, time.Duration(cfg.Redis.JobTTL))
		urlCache = cache.NewURLCache(rdb, time.Duration(cfg.Redis.CacheTTLSeconds)*time.Second)
	}

	// 5. Setup JWT Validator
	var validator *auth.JWTValidator
	validator, err = auth.NewJWTValidator(cfg.JWT.PublicKeyPath, rdb, cfg.Redis.BlocklistPrefix)
	if err != nil {
		logger.Error("failed to initialize JWT validator", "key_path", cfg.JWT.PublicKeyPath, "err", err)
		os.Exit(1)
	}

	// 6. Setup RSS Reader library
	reader := rssreader.New(
		rssreader.WithConcurrencyLimit(cfg.RssReader.ConcurrencyLimit),
		rssreader.WithMaxBodySize(cfg.RssReader.MaxBodySize),
	)

	// 7. Setup Background Workers & Publishers (only in full mode)
	var client *rabbitmq.Client
	var publisher *rabbitmq.Publisher
	var backgroundWorker *worker.Worker
	workerCtx, workerCancel := context.WithCancel(context.Background())
	defer workerCancel()

	if !isFallback {
		client = rabbitmq.NewClient(cfg.RabbitMQ.URL, cfg.RabbitMQ.Prefetch, logger)
		err = client.Connect(workerCtx)
		if err != nil {
			logger.Warn("RabbitMQ is unreachable at boot, starting in dynamic fallback. Connection will recover in background.", "err", err)
			client.TriggerRecovery()
		} else {
			ch, topologyErr := client.Channel()
			if topologyErr == nil {
				if err := rabbitmq.DeclareTopology(ch, cfg); err != nil {
					logger.Error("failed to declare RabbitMQ topology", "err", err)
					os.Exit(1)
				}
			}
		}

		publisher = rabbitmq.NewPublisher(client, cfg)
		consumer := rabbitmq.NewConsumer(client, cfg, logger)
		processor := worker.NewProcessor(reader, store, rdb, urlCache, publisher, logger)
		backgroundWorker = worker.NewWorker(client, consumer, processor, cfg, logger)

		client.StartRecoveryLoop(workerCtx, func(recoveryCtx context.Context) error {
			ch, err := client.Channel()
			if err != nil {
				return err
			}
			return rabbitmq.DeclareTopology(ch, cfg)
		})

		go func() {
			if err := backgroundWorker.Start(workerCtx); err != nil && err != context.Canceled {
				logger.Error("background worker crashed", "err", err)
			}
		}()
	}

	// 8. Start HTTP Server
	srv := server.NewServer(
		cfg.Server.Port,
		time.Duration(cfg.Server.ReadTimeout),
		time.Duration(cfg.Server.WriteTimeout),
		time.Duration(cfg.Server.IdleTimeout),
		reader,
		store,
		publisher,
		urlCache,
		validator,
		isFallback,
		logger,
	)

	go func() {
		if err := srv.Start(); err != nil {
			logger.Error("HTTP server failed to start", "err", err)
			os.Exit(1)
		}
	}()

	// 9. Trapping termination signals for graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	logger.Info("shutting down service gracefully")
	workerCancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("failed to shutdown HTTP server gracefully", "err", err)
	}

	if client != nil {
		client.Close()
	}

	if rdb != nil {
		rdb.Close()
	}

	logger.Info("service stopped")
}
