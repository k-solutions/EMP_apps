package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/emarchant/rssservice/internal/config"
	amqp "github.com/rabbitmq/amqp091-go"
)

type Publisher struct {
	client *Client
	cfg    *config.Config
}

func NewPublisher(client *Client, cfg *config.Config) *Publisher {
	return &Publisher{
		client: client,
		cfg:    cfg,
	}
}

// PublishCommand enqueues a parsing job command to the rss_commands exchange.
func (p *Publisher) PublishCommand(ctx context.Context, jobID string, urls []string) error {
	ch, err := p.client.Channel()
	if err != nil {
		return fmt.Errorf("failed to get channel for publishing command: %w", err)
	}

	payload := struct {
		JobID string   `json:"job_id"`
		URLs  []string `json:"urls"`
	}{
		JobID: jobID,
		URLs:  urls,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal command payload: %w", err)
	}

	return ch.PublishWithContext(
		ctx,
		p.cfg.RabbitMQ.Exchanges.Commands,
		p.cfg.RabbitMQ.RoutingKeys.CommandsWorker,
		false,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent, // persistent: true
			Body:         data,
		},
	)
}

// PublishResult pushes a parsing outcome to the rss_results exchange.
func (p *Publisher) PublishResult(ctx context.Context, jobID string, status string, items interface{}, errors interface{}) error {
	ch, err := p.client.Channel()
	if err != nil {
		return fmt.Errorf("failed to get channel for publishing result: %w", err)
	}

	payload := struct {
		JobID  string      `json:"job_id"`
		Status string      `json:"status"`
		Items  interface{} `json:"items,omitempty"`
		Errors interface{} `json:"errors,omitempty"`
	}{
		JobID:  jobID,
		Status: status,
		Items:  items,
		Errors: errors,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal result payload: %w", err)
	}

	return ch.PublishWithContext(
		ctx,
		p.cfg.RabbitMQ.Exchanges.Results,
		p.cfg.RabbitMQ.RoutingKeys.ResultsRails,
		false,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent, // persistent: true
			Body:         data,
		},
	)
}
