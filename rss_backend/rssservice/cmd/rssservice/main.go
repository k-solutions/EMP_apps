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
	defer func() {
		if r := recover(); r != nil {
			slog.Error("service crashed due to an unhandled panic", "panic", r)
			os.Exit(1)
		}
	}()

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

	// 3. Connect to Redis (Mandatory)
	opt, err := redis.ParseURL(cfg.Redis.DSN)
	if err != nil {
		logger.Error("invalid Redis DSN", "dsn", cfg.Redis.DSN, "err", err)
		os.Exit(1)
	}

	rdb := redis.NewClient(opt)
	pingCtx, pingCancel := context.WithTimeout(context.Background(), 2*time.Second)
	if err := rdb.Ping(pingCtx).Err(); err != nil {
		logger.Error("failed to ping Redis (Redis is required to start)", "err", err)
		pingCancel()
		os.Exit(1)
	}
	pingCancel()

	// 4. Setup JobStore & Cache
	store := jobstore.NewRedisJobStore(rdb, time.Duration(cfg.Redis.JobTTL))
	urlCache := cache.NewURLCache(rdb, time.Duration(cfg.Redis.CacheTTLSeconds)*time.Second)

	// 5. Setup JWT Validator
	validator, err := auth.NewJWTValidator(cfg.JWT.PublicKeyPath, rdb, cfg.Redis.BlocklistPrefix)
	if err != nil {
		logger.Error("failed to initialize JWT validator", "key_path", cfg.JWT.PublicKeyPath, "err", err)
		os.Exit(1)
	}

	// 6. Setup RSS Reader library
	reader := rssreader.New(
		rssreader.WithConcurrencyLimit(cfg.RssReader.ConcurrencyLimit),
		rssreader.WithMaxBodySize(cfg.RssReader.MaxBodySize),
	)

	// 7. Connect to RabbitMQ (Optional at boot time — will recover in background)
	workerCtx, workerCancel := context.WithCancel(context.Background())
	defer workerCancel()

	client := rabbitmq.NewClient(cfg.RabbitMQ.URL, cfg.RabbitMQ.Prefetch, logger)
	err = client.Connect(workerCtx)
	if err != nil {
		logger.Warn("RabbitMQ is unreachable at boot. Starting REST API in dynamic fallback. Connection will recover in background.", "err", err)
		client.TriggerRecovery()
	} else {
		ch, err := client.Channel()
		if err == nil {
			if err := rabbitmq.DeclareTopology(ch, cfg); err != nil {
				logger.Error("failed to declare RabbitMQ topology", "err", err)
				os.Exit(1)
			}
		}
	}

	// 8. Setup background worker and recovery loops
	publisher := rabbitmq.NewPublisher(client, cfg)
	consumer := rabbitmq.NewConsumer(client, cfg, logger)
	processor := worker.NewProcessor(reader, store, rdb, urlCache, publisher, logger)
	backgroundWorker := worker.NewWorker(client, consumer, processor, cfg, logger)

	client.StartRecoveryLoop(workerCtx, func(recoveryCtx context.Context) error {
		ch, err := client.Channel()
		if err != nil {
			return err
		}
		return rabbitmq.DeclareTopology(ch, cfg)
	})

	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("recovered from panic in background worker", "panic", r)
			}
		}()
		if err := backgroundWorker.Start(workerCtx); err != nil && err != context.Canceled {
			logger.Error("background worker crashed", "err", err)
		}
	}()

	// 9. Start HTTP Server
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
		false, // isFallback is always false (booted in full mode)
		logger,
	)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("recovered from panic in HTTP server boot", "panic", r)
			}
		}()
		if err := srv.Start(); err != nil {
			logger.Error("HTTP server failed to start", "err", err)
			os.Exit(1)
		}
	}()

	// 10. Trapping termination signals for immediate exit
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGHUP)
	sig := <-stop

	logger.Info("received termination signal, stopping service immediately", "signal", sig.String())
	os.Exit(0)
}
