package rabbitmq

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/emarchant/rssservice/internal/config"
	amqp "github.com/rabbitmq/amqp091-go"
)

type Client struct {
	url           string
	prefetchCount int
	logger        *slog.Logger

	mu      sync.RWMutex
	conn    *amqp.Connection
	channel *amqp.Channel
	closed  bool

	reconnectChan chan struct{}
}

func NewClient(url string, prefetchCount int, logger *slog.Logger) *Client {
	return &Client{
		url:           url,
		prefetchCount: prefetchCount,
		logger:        logger,
		reconnectChan: make(chan struct{}, 1),
	}
}

func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return fmt.Errorf("client already closed")
	}

	conn, err := amqp.Dial(c.url)
	if err != nil {
		return fmt.Errorf("failed to dial rabbitmq: %w", err)
	}

	channel, err := conn.Channel()
	if err != nil {
		conn.Close()
		return fmt.Errorf("failed to open channel: %w", err)
	}

	if err := channel.Qos(c.prefetchCount, 0, false); err != nil {
		channel.Close()
		conn.Close()
		return fmt.Errorf("failed to set channel QoS: %w", err)
	}

	c.conn = conn
	c.channel = channel

	go c.handleDisconnects(conn, channel)

	return nil
}

func (c *Client) handleDisconnects(conn *amqp.Connection, channel *amqp.Channel) {
	connClose := conn.NotifyClose(make(chan *amqp.Error, 1))
	chanClose := channel.NotifyClose(make(chan *amqp.Error, 1))

	select {
	case err := <-connClose:
		if err != nil {
			c.logger.Error("RabbitMQ connection closed, triggering reconnect", "err", err)
		}
	case err := <-chanClose:
		if err != nil {
			c.logger.Error("RabbitMQ channel closed, triggering reconnect", "err", err)
		}
	}

	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	if c.channel != nil {
		c.channel.Close()
	}
	if c.conn != nil {
		c.conn.Close()
	}
	c.channel = nil
	c.conn = nil
	c.mu.Unlock()

	select {
	case c.reconnectChan <- struct{}{}:
	default:
	}
}

func (c *Client) StartRecoveryLoop(ctx context.Context, onReconnect func(ctx context.Context) error) {
	go func() {
		backoff := 1 * time.Second
		maxBackoff := 60 * time.Second

		for {
			select {
			case <-ctx.Done():
				return
			case <-c.reconnectChan:
				c.logger.Info("Starting RabbitMQ connection recovery")
				for {
					select {
					case <-ctx.Done():
						return
					default:
					}

					err := c.Connect(ctx)
					if err == nil {
						c.logger.Info("RabbitMQ successfully reconnected")
						backoff = 1 * time.Second
						if onReconnect != nil {
							if err := onReconnect(ctx); err != nil {
								c.logger.Error("reconnect callback failed, continuing reconnection loop", "err", err)
								c.Close()
								time.Sleep(backoff)
								backoff = minDuration(backoff*2, maxBackoff)
								continue
							}
						}
						break
					}

					c.logger.Error("RabbitMQ reconnection failed, retrying...", "err", err, "backoff", backoff)
					time.Sleep(backoff)
					backoff = minDuration(backoff*2, maxBackoff)
				}
			}
		}
	}()
}

// TriggerRecovery manually signals the recovery loop to attempt a reconnection.
func (c *Client) TriggerRecovery() {
	select {
	case c.reconnectChan <- struct{}{}:
	default:
	}
}

func (c *Client) Channel() (*amqp.Channel, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.channel == nil {
		return nil, fmt.Errorf("rabbitmq channel is not established")
	}
	return c.channel, nil
}

func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.closed = true
	var err error
	if c.channel != nil {
		err = c.channel.Close()
	}
	if c.conn != nil {
		if closeErr := c.conn.Close(); closeErr != nil {
			err = closeErr
		}
	}
	c.channel = nil
	c.conn = nil
	return err
}

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

func DeclareTopology(ch *amqp.Channel, cfg *config.Config) error {
	// 1. Declare exchanges
	if err := ch.ExchangeDeclare(
		cfg.RabbitMQ.Exchanges.Commands,
		"direct",
		true,  // durable
		false, // auto-deleted
		false, // internal
		false, // no-wait
		nil,   // arguments
	); err != nil {
		return fmt.Errorf("failed to declare commands exchange: %w", err)
	}

	if err := ch.ExchangeDeclare(
		cfg.RabbitMQ.Exchanges.Retry,
		"direct",
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		return fmt.Errorf("failed to declare retry exchange: %w", err)
	}

	if err := ch.ExchangeDeclare(
		cfg.RabbitMQ.Exchanges.Results,
		"direct",
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		return fmt.Errorf("failed to declare results exchange: %w", err)
	}

	// 2. Declare worker queue (with DLX config)
	workerArgs := amqp.Table{
		"x-dead-letter-exchange":    cfg.RabbitMQ.Exchanges.Retry,
		"x-dead-letter-routing-key": cfg.RabbitMQ.RoutingKeys.CommandsWorker,
	}
	if _, err := ch.QueueDeclare(
		cfg.RabbitMQ.Queues.Worker,
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		workerArgs,
	); err != nil {
		return fmt.Errorf("failed to declare worker queue: %w", err)
	}

	if err := ch.QueueBind(
		cfg.RabbitMQ.Queues.Worker,
		cfg.RabbitMQ.RoutingKeys.CommandsWorker,
		cfg.RabbitMQ.Exchanges.Commands,
		false,
		nil,
	); err != nil {
		return fmt.Errorf("failed to bind worker queue: %w", err)
	}

	// 3. Declare retry wait queue
	waitArgs := amqp.Table{
		"x-message-ttl":             int32(30000), // 30s
		"x-dead-letter-exchange":    cfg.RabbitMQ.Exchanges.Commands,
		"x-dead-letter-routing-key": cfg.RabbitMQ.RoutingKeys.CommandsWorker,
	}
	if _, err := ch.QueueDeclare(
		cfg.RabbitMQ.Queues.Wait30s,
		true,
		false,
		false,
		false,
		waitArgs,
	); err != nil {
		return fmt.Errorf("failed to declare wait_30s queue: %w", err)
	}

	if err := ch.QueueBind(
		cfg.RabbitMQ.Queues.Wait30s,
		cfg.RabbitMQ.RoutingKeys.CommandsWorker,
		cfg.RabbitMQ.Exchanges.Retry,
		false,
		nil,
	); err != nil {
		return fmt.Errorf("failed to bind wait_30s queue: %w", err)
	}

	// 4. Declare failed queue
	if _, err := ch.QueueDeclare(
		cfg.RabbitMQ.Queues.Failed,
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		return fmt.Errorf("failed to declare failed queue: %w", err)
	}

	// 5. Declare and bind results queue
	if _, err := ch.QueueDeclare(
		"rss_results_rails",
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		return fmt.Errorf("failed to declare results queue: %w", err)
	}

	if err := ch.QueueBind(
		"rss_results_rails",
		cfg.RabbitMQ.RoutingKeys.ResultsRails,
		cfg.RabbitMQ.Exchanges.Results,
		false,
		nil,
	); err != nil {
		return fmt.Errorf("failed to bind results queue: %w", err)
	}

	return nil
}
