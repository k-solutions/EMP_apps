package worker

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/emarchant/rssservice/internal/config"
	"github.com/emarchant/rssservice/internal/rabbitmq"
)

type Worker struct {
	client    *rabbitmq.Client
	consumer  *rabbitmq.Consumer
	processor *Processor
	cfg       *config.Config
	logger    *slog.Logger
}

func NewWorker(
	client *rabbitmq.Client,
	consumer *rabbitmq.Consumer,
	processor *Processor,
	cfg *config.Config,
	logger *slog.Logger,
) *Worker {
	return &Worker{
		client:    client,
		consumer:  consumer,
		processor: processor,
		cfg:       cfg,
		logger:    logger,
	}
}

func (w *Worker) Start(ctx context.Context) error {
	w.logger.Info("starting background worker loop")

	// Start daily sweep routine
	go w.startDailySweep(ctx)

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("shutting down background worker loop")
			return ctx.Err()
		default:
		}

		w.logger.Info("starting RabbitMQ consumption")
		err := w.consumer.Start(ctx, func(ctx context.Context, payload rabbitmq.CommandPayload) error {
			return w.processor.Process(ctx, payload.JobID, payload.URLs)
		})

		if err != nil {
			if err == context.Canceled {
				return nil
			}
			w.logger.Error("RabbitMQ consumer error, restarting consumption in 2 seconds", "err", err)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(2 * time.Second):
			}
		}
	}
}

func (w *Worker) startDailySweep(ctx context.Context) {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	// Run once at startup as well
	w.sweepFailedQueue(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.sweepFailedQueue(ctx)
		}
	}
}

func (w *Worker) sweepFailedQueue(ctx context.Context) {
	w.logger.Info("running automated sweep routine on rss_commands_failed")

	ch, err := w.client.Channel()
	if err != nil {
		w.logger.Error("daily sweep: failed to get channel", "err", err)
		return
	}

	clearedCount := 0
	errorTrends := make(map[string]int)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		msg, ok, err := ch.Get(w.cfg.RabbitMQ.Queues.Failed, true)
		if err != nil {
			w.logger.Error("daily sweep: failed to get message from failed queue", "err", err)
			break
		}
		if !ok {
			break
		}

		clearedCount++
		var payload struct {
			JobID string   `json:"job_id"`
			URLs  []string `json:"urls"`
		}
		if err := json.Unmarshal(msg.Body, &payload); err == nil {
			errStr := "unknown_error"
			if msg.Headers != nil {
				if e, ok := msg.Headers["error"].(string); ok {
					errStr = e
				}
			}
			errorTrends[errStr]++
			w.logger.Warn("cleared failed job from rss_commands_failed during daily sweep", "job_id", payload.JobID, "urls", payload.URLs, "err", errStr)
		} else {
			errorTrends["malformed_json"]++
			w.logger.Warn("cleared malformed failed message from rss_commands_failed during daily sweep")
		}
	}

	w.logger.Info("completed daily sweep routine on rss_commands_failed", "cleared_count", clearedCount, "error_trends", errorTrends)
}
