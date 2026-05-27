package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/emarchant/rssservice/internal/config"
	amqp "github.com/rabbitmq/amqp091-go"
)

type Consumer struct {
	client *Client
	cfg    *config.Config
	logger *slog.Logger
}

func NewConsumer(client *Client, cfg *config.Config, logger *slog.Logger) *Consumer {
	return &Consumer{
		client: client,
		cfg:    cfg,
		logger: logger,
	}
}

type CommandPayload struct {
	JobID string   `json:"job_id"`
	URLs  []string `json:"urls"`
}

// Start consuming commands from the worker queue
func (c *Consumer) Start(ctx context.Context, handler func(ctx context.Context, payload CommandPayload) error) error {
	ch, err := c.client.Channel()
	if err != nil {
		return fmt.Errorf("failed to get channel for consuming: %w", err)
	}

	tag := c.cfg.RabbitMQ.ConsumerTag
	if tag != "" {
		tag = fmt.Sprintf("%s-%d", tag, time.Now().UnixNano())
	}

	deliveries, err := ch.Consume(
		c.cfg.RabbitMQ.Queues.Worker,
		tag,
		false, // autoAck = false
		false, // exclusive
		false, // noLocal
		false, // noWait
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to start consuming from queue %s: %w", c.cfg.RabbitMQ.Queues.Worker, err)
	}

	c.logger.Info("RabbitMQ consumer started", "queue", c.cfg.RabbitMQ.Queues.Worker)

	for {
		select {
		case <-ctx.Done():
			c.logger.Info("RabbitMQ consumer context cancelled, shutting down consumer")
			return ctx.Err()
		case d, ok := <-deliveries:
			if !ok {
				c.logger.Warn("RabbitMQ delivery channel closed")
				return fmt.Errorf("delivery channel closed")
			}

			// Process delivery
			go c.processDelivery(ctx, ch, d, handler)
		}
	}
}

func (c *Consumer) processDelivery(ctx context.Context, ch *amqp.Channel, d amqp.Delivery, handler func(ctx context.Context, payload CommandPayload) error) {
	// 1. Decode payload
	var payload CommandPayload
	if err := json.Unmarshal(d.Body, &payload); err != nil {
		c.logger.Error("failed to decode message payload, discarding message", "err", err)
		_ = d.Ack(false)
		return
	}

	// 2. Check retry count in x-death
	retryCount := getRetryCount(d.Headers)
	c.logger.Debug("received command message", "job_id", payload.JobID, "retry_count", retryCount)

	if retryCount >= 3 {
		c.logger.Warn("message exceeded maximum 3 retries, dead-lettering to failed queue", "job_id", payload.JobID, "retries", retryCount)
		
		err := ch.PublishWithContext(
			ctx,
			"", // default exchange
			c.cfg.RabbitMQ.Queues.Failed,
			false,
			false,
			amqp.Publishing{
				ContentType:  "application/json",
				DeliveryMode: amqp.Persistent,
				Body:         d.Body,
				Headers: amqp.Table{
					"error": "exceeded maximum 3 retries",
				},
			},
		)
		if err != nil {
			c.logger.Error("failed to publish to failed queue", "job_id", payload.JobID, "err", err)
			_ = d.Nack(false, true)
			return
		}

		_ = d.Ack(false)
		return
	}

	// 3. Execute handler
	err := handler(ctx, payload)
	if err != nil {
		c.logger.Error("handler processing failed, rejecting message for retry", "job_id", payload.JobID, "err", err)
		_ = d.Nack(false, false)
		return
	}

	// 4. Manual ACK upon success
	if err := d.Ack(false); err != nil {
		c.logger.Error("failed to ACK message", "job_id", payload.JobID, "err", err)
	}
}

func getRetryCount(headers amqp.Table) int64 {
	if headers == nil {
		return 0
	}
	xDeath, ok := headers["x-death"]
	if !ok {
		return 0
	}

	slice, ok := xDeath.([]interface{})
	if !ok {
		return 0
	}

	var maxCount int64 = 0
	for _, entry := range slice {
		table, ok := entry.(amqp.Table)
		if !ok {
			continue
		}
		if countVal, ok := table["count"]; ok {
			if count, ok := countVal.(int64); ok {
				if count > maxCount {
					maxCount = count
				}
			} else if count, ok := countVal.(int32); ok {
				if int64(count) > maxCount {
					maxCount = int64(count)
				}
			}
		}
	}
	return maxCount
}
