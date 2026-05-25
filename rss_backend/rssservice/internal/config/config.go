package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

// Duration is a wrapper around time.Duration to support YAML unmarshaling of duration strings.
type Duration time.Duration

func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err != nil {
		return err
	}
	dur, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	*d = Duration(dur)
	return nil
}

type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Redis     RedisConfig     `yaml:"redis"`
	RabbitMQ  RabbitMQConfig  `yaml:"rabbitmq"`
	JWT       JWTConfig       `yaml:"jwt"`
	RssReader RssReaderConfig `yaml:"rssreader"`
	Log       LogConfig       `yaml:"log"`
}

type ServerConfig struct {
	Port         int      `yaml:"port"`
	ReadTimeout  Duration `yaml:"read_timeout"`
	WriteTimeout Duration `yaml:"write_timeout"`
	IdleTimeout  Duration `yaml:"idle_timeout"`
}

type RedisConfig struct {
	DSN             string   `yaml:"dsn"`
	JobTTL          Duration `yaml:"job_ttl"`
	CacheTTLSeconds int      `yaml:"cache_ttl_seconds"`
	BlocklistPrefix string   `yaml:"blocklist_prefix"`
}

type RabbitMQConfig struct {
	URL         string            `yaml:"url"`
	Exchanges   ExchangesConfig   `yaml:"exchanges"`
	Queues      QueuesConfig      `yaml:"queues"`
	RoutingKeys RoutingKeysConfig `yaml:"routing_keys"`
	ConsumerTag string            `yaml:"consumer_tag"`
	Prefetch    int               `yaml:"prefetch"`
}

type ExchangesConfig struct {
	Commands string `yaml:"commands"`
	Results  string `yaml:"results"`
	Retry    string `yaml:"retry"`
}

type QueuesConfig struct {
	Worker  string `yaml:"worker"`
	Wait30s string `yaml:"wait_30s"`
	Failed  string `yaml:"failed"`
}

type RoutingKeysConfig struct {
	CommandsWorker string `yaml:"commands_worker"`
	ResultsRails   string `yaml:"results_rails"`
}

type JWTConfig struct {
	PublicKeyPath string `yaml:"public_key_path"`
}

type RssReaderConfig struct {
	ConcurrencyLimit int   `yaml:"concurrency_limit"`
	MaxBodySize      int64 `yaml:"max_body_size"`
}

type LogConfig struct {
	Level string `yaml:"level"`
}

func getEnv(key string, fallback string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if val, ok := os.LookupEnv(key); ok {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return fallback
}

func getEnvInt64(key string, fallback int64) int64 {
	if val, ok := os.LookupEnv(key); ok {
		if i, err := strconv.ParseInt(val, 10, 64); err == nil {
			return i
		}
	}
	return fallback
}

func getEnvDuration(key string, fallback Duration) Duration {
	if val, ok := os.LookupEnv(key); ok {
		if d, err := time.ParseDuration(val); err == nil {
			return Duration(d)
		}
	}
	return fallback
}

// LoadConfig loads the YAML configuration from file and applies environment overrides.
func LoadConfig(path string) (*Config, error) {
	var cfg Config

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse yaml config: %w", err)
	}

	// Environment overrides
	cfg.Server.Port = getEnvInt("SERVER_PORT", cfg.Server.Port)
	cfg.Server.ReadTimeout = getEnvDuration("SERVER_READ_TIMEOUT", cfg.Server.ReadTimeout)
	cfg.Server.WriteTimeout = getEnvDuration("SERVER_WRITE_TIMEOUT", cfg.Server.WriteTimeout)
	cfg.Server.IdleTimeout = getEnvDuration("SERVER_IDLE_TIMEOUT", cfg.Server.IdleTimeout)

	cfg.Redis.DSN = getEnv("REDIS_DSN", cfg.Redis.DSN)
	cfg.Redis.JobTTL = getEnvDuration("REDIS_JOB_TTL", cfg.Redis.JobTTL)
	cfg.Redis.CacheTTLSeconds = getEnvInt("REDIS_CACHE_TTL_SECONDS", cfg.Redis.CacheTTLSeconds)
	cfg.Redis.BlocklistPrefix = getEnv("REDIS_BLOCKLIST_PREFIX", cfg.Redis.BlocklistPrefix)

	cfg.RabbitMQ.URL = getEnv("RABBITMQ_URL", cfg.RabbitMQ.URL)
	cfg.RabbitMQ.Exchanges.Commands = getEnv("RABBITMQ_EXCHANGES_COMMANDS", cfg.RabbitMQ.Exchanges.Commands)
	cfg.RabbitMQ.Exchanges.Results = getEnv("RABBITMQ_EXCHANGES_RESULTS", cfg.RabbitMQ.Exchanges.Results)
	cfg.RabbitMQ.Exchanges.Retry = getEnv("RABBITMQ_EXCHANGES_RETRY", cfg.RabbitMQ.Exchanges.Retry)
	cfg.RabbitMQ.Queues.Worker = getEnv("RABBITMQ_QUEUES_WORKER", cfg.RabbitMQ.Queues.Worker)
	cfg.RabbitMQ.Queues.Wait30s = getEnv("RABBITMQ_QUEUES_WAIT_30S", cfg.RabbitMQ.Queues.Wait30s)
	cfg.RabbitMQ.Queues.Failed = getEnv("RABBITMQ_QUEUES_FAILED", cfg.RabbitMQ.Queues.Failed)
	cfg.RabbitMQ.RoutingKeys.CommandsWorker = getEnv("RABBITMQ_ROUTING_KEYS_COMMANDS_WORKER", cfg.RabbitMQ.RoutingKeys.CommandsWorker)
	cfg.RabbitMQ.RoutingKeys.ResultsRails = getEnv("RABBITMQ_ROUTING_KEYS_RESULTS_RAILS", cfg.RabbitMQ.RoutingKeys.ResultsRails)
	cfg.RabbitMQ.ConsumerTag = getEnv("RABBITMQ_CONSUMER_TAG", cfg.RabbitMQ.ConsumerTag)
	cfg.RabbitMQ.Prefetch = getEnvInt("RABBITMQ_PREFETCH", cfg.RabbitMQ.Prefetch)

	cfg.JWT.PublicKeyPath = getEnv("JWT_PUBLIC_KEY_PATH", cfg.JWT.PublicKeyPath)

	cfg.RssReader.ConcurrencyLimit = getEnvInt("RSSREADER_CONCURRENCY_LIMIT", cfg.RssReader.ConcurrencyLimit)
	cfg.RssReader.MaxBodySize = getEnvInt64("RSSREADER_MAX_BODY_SIZE", cfg.RssReader.MaxBodySize)

	cfg.Log.Level = getEnv("LOG_LEVEL", cfg.Log.Level)

	return &cfg, nil
}
