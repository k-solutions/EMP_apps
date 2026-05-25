# RSS Reader Service

A high-performance, asynchronous RSS/Atom feed parsing service built in Go. It supports JWT authentication (ES256), a dedicated **Redis Transient URL Caching** tier, **RabbitMQ AMQP 0-9-1** messaging broker pipelines with durable retry mechanisms, and a **High-Availability Dual Mode** offering seamless synchronous fallbacks during broker outages.

---

## Technical Architecture

The service runs a single-process dual architecture combining the REST API and continuous worker loops:

```
[ Client ] ──( HTTPS + JWT )──> [ Chi HTTP Server ]
                                      │
                         (Broker OK?) ├── Yes ──> [ RabbitMQ: rss_commands ] ──> [ Worker Loop ]
                                      │                                                │
                                      └── No ───> [ Synchronous Fallback ]            │
                                                           │                           │
                                                           v                           v
                                                   [ Redis Cache ] <───────────> [ Parse Feed ]
```

- **Primary Message Bus**: RabbitMQ direct exchanges with persistent delivery configurations and manual ACKs.
- **Fail-Safe Retries**: Dead Letter Exchange (DLX) "Wait Queue" pattern routing failures through a 30-second TTL delay queue. Drops to `rss_commands_failed` after 3 consecutive failures.
- **URL Output Cache**: Fast Redis-backed uniform caching using MD5 keys (`cache:url:<hash>`) with a strict, explicit TTL window to suppress unnecessary outbound network amplification.
- **High-Availability Dual Mode**: Attempts RabbitMQ asynchronous queueing by default. Falls back instantly and transparently to **Inline Synchronous Processing** on queue/broker channel exceptions, returning a `200 OK` with full results immediately.

---

## Prerequisites

- **Go 1.26+** (compatible down to 1.23)
- **Redis 7+**
- **RabbitMQ 3.8+**
- **Podman** or Docker (for containerized execution)
- **OpenSSL** (to generate ECDSA key pairs)

---

## 1. Generating an EC Key Pair

The service uses **ES256** (ECDSA with P-256) for verifying JWTs. Generate a key pair using OpenSSL:

```bash
# Generate a private key
openssl ecparam -name prime256v1 -genkey -noout -out keys/ec_private.pem

# Extract the public key
openssl ec -in keys/ec_private.pem -pubout -out keys/ec_public.pem
```

---

## 2. Configuration (`config.yaml` + env overrides)

The default configuration file is `config.yaml`. Every key can be overridden using an uppercase, `_`-separated environment variable mapping to its path:

### Configuration Reference

| Yaml Key | Env Variable Override | Description | Default |
| --- | --- | --- | --- |
| `server.port` | `SERVER_PORT` | HTTP server port | `8080` |
| `server.read_timeout` | `SERVER_READ_TIMEOUT` | HTTP read timeout | `15s` |
| `redis.dsn` | `REDIS_DSN` | Redis connection DSN | `redis://localhost:6379/0` |
| `redis.job_ttl` | `REDIS_JOB_TTL` | Retention duration for Redis jobs | `1h` |
| `redis.cache_ttl_seconds` | `REDIS_CACHE_TTL_SECONDS` | Cache duration for parsed URL structures | `3600` (1 hour) |
| `redis.blocklist_prefix` | `REDIS_BLOCKLIST_PREFIX` | JWT blocklist prefix keys in Redis | `jwt:blocklist:` |
| `rabbitmq.url` | `RABBITMQ_URL` | RabbitMQ AMQP connection URL | `amqp://guest:guest@localhost:5672/` |
| `rabbitmq.exchanges.commands` | `RABBITMQ_EXCHANGES_COMMANDS` | Exchange for commands | `rss_commands` |
| `rabbitmq.exchanges.results` | `RABBITMQ_EXCHANGES_RESULTS` | Exchange for processing results | `rss_results` |
| `rabbitmq.exchanges.retry` | `RABBITMQ_EXCHANGES_RETRY` | Dead letter / retry exchange | `rss_commands_retry` |
| `rabbitmq.queues.worker` | `RABBITMQ_QUEUES_WORKER` | Active command worker queue | `rss_commands_worker` |
| `rabbitmq.queues.wait_30s` | `RABBITMQ_QUEUES_WAIT_30S` | Delay waiting queue | `rss_commands_wait_30s` |
| `rabbitmq.queues.failed` | `RABBITMQ_QUEUES_FAILED` | Target queue for dead lettered drops | `rss_commands_failed` |
| `rabbitmq.routing_keys.commands_worker` | `RABBITMQ_ROUTING_KEYS_COMMANDS_WORKER` | Routing key for command consumer | `rss_commands_worker` |
| `rabbitmq.routing_keys.results_rails` | `RABBITMQ_ROUTING_KEYS_RESULTS_RAILS` | Routing key for results subscriber | `rss_results_rails` |
| `rabbitmq.consumer_tag` | `RABBITMQ_CONSUMER_TAG` | Active consumer identity | `rss-go-worker-1` |
| `rabbitmq.prefetch` | `RABBITMQ_PREFETCH` | Channel prefetch limits | `10` |
| `jwt.public_key_path` | `JWT_PUBLIC_KEY_PATH` | ES256 Public key PEM path | `./keys/ec_public.pem` |
| `rssreader.concurrency_limit` | `RSSREADER_CONCURRENCY_LIMIT` | Concurrency bounds inside feed parser | `10` |
| `rssreader.max_body_size` | `RSSREADER_MAX_BODY_SIZE` | Maximum body read limit per fetch | `10485760` (10 MB) |
| `log.level` | `LOG_LEVEL` | Logging level (`debug`, `info`, `warn`, `error`) | `info` |

---

## 3. Building and Running

### Building locally
```bash
go build -o rssservice ./cmd/rssservice
```

### Building with Podman
Run from the workspace root:
```bash
podman build -t rssservice -f rssservice/Dockerfile .
```

### Running locally
Ensure Redis and RabbitMQ are running, then start the service:
```bash
./rssservice
```

#### High-Availability Dual Mode (Dynamic Fallback)
If RabbitMQ is down at boot or goes offline during operation:
- The service will automatically boot or stay alive, logging a warning.
- `POST /parse` catches connection exceptions instantly and executes **Inline Synchronous Processing** (checking the Redis cache first, fetching misses, and saving the results).
- Returns `200 OK` with full parsed results inline immediately.
- The RabbitMQ client attempts automatic connection recovery loops in the background with exponential backoff, restoring full asynchronous mode once the broker recovers.

---

## 4. REST API Usage

All endpoints except `/health` and `/docs` require a Bearer JWT:
```
Authorization: Bearer <ES256-signed JWT>
```

### `GET /health`
```bash
curl -s http://localhost:8080/health
```

### `GET /docs`
Open `http://localhost:8080/docs` in your browser to view the interactive Swagger UI (served completely offline).

### `POST /parse`
**Request:**
```bash
curl -i -X POST http://localhost:8080/parse \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"urls": ["https://feeds.bbci.co.uk/news/rss.xml"]}'
```

**Response (Asynchronous Ingestion Mode — `202 Accepted`):**
```json
{ "job_id": "01J3K..." }
```

**Response (Dynamic Synchronous Fallback — `200 OK`):**
```json
{
  "job_id": "01J3K...",
  "status": "done",
  "items": [
    {
      "title": "Example Feed Item",
      "source": "BBC News",
      "source_url": "https://feeds.bbci.co.uk/news/rss.xml",
      "link": "https://www.bbc.co.uk/news/12345",
      "publish_date": "2026-05-25",
      "description": "Content description text."
    }
  ],
  "errors": []
}
```

### `GET /jobs/{id}`
**Request:**
```bash
curl -s http://localhost:8080/jobs/01J3K... \
  -H "Authorization: Bearer <token>"
```

**Response (`200 OK`):**
```json
{
  "job_id": "01J3K...",
  "status": "done",
  "items": [
    {
      "title": "Example Feed Item",
      "source": "BBC News",
      "source_url": "https://feeds.bbci.co.uk/news/rss.xml",
      "link": "https://www.bbc.co.uk/news/12345",
      "publish_date": "2026-05-25",
      "description": "Content description text."
    }
  ],
  "errors": []
}
```

---

## 5. JWT Structure & Issuance

Tokens must be pre-issued externally. The service expects:
- **Algorithm**: `ES256`
- **Public Key**: Loaded from `jwt.public_key_path`
- **Claims**: standard `jwt.RegisteredClaims` (`jti` for blocklist checks, `exp` for expirations, `sub`).

---

## 6. Running Tests

Run all unit tests:
```bash
go test -v ./...
```

Run integration tests (requires local running instances of Redis at `localhost:6379` and RabbitMQ at `amqp://localhost:5672/`):
```bash
go test -v -tags=integration ./...
```
