# RSS Reader Service — Implementation Plan

## All Decisions

| Concern | Decision |
|---|---|
| **Service structure** | Single binary — API + worker in one process |
| **Configuration** | YAML/TOML config file + environment variable overrides |
| **Primary Message Bus** | RabbitMQ — Direct exchanges, AMQP 0-9-1 (Primary asynchronous execution path) |
| **RabbitMQ Properties** | Durable Direct Exchanges (`rss_commands`, `rss_results`) + Explicit Message Durability (`persistent: true`) + Manual Acks |
| **Failure Handling** | Dead Letter Exchange (DLX) "Wait Queue" pattern with maximum 3 TTL-based retries. Daily automated cleanup schedule on `rss_commands_failed`. |
| **State Management** | Shared Redis Store — Tracking `job:<id>` execution states (`pending`, `processing`, `done`, `failed`) |
| **URL Result Caching** | Shared Redis Cache — Caches successful JSON parsing results under `cache:url:<hash>` enforced with a strict, explicit TTL. |
| **job_id ownership** | Rails generates the ULID; Go worker echoes it back |
| **JWT algorithm** | ES256 (elliptic curve, `crypto/ecdsa`) |
| **JWT revocation** | Stateful — blocklist stored in Redis |
| **JWT issuance** | Tokens pre-issued externally; service only validates |
| **OpenAPI docs** | Swagger UI served by the service at `GET /docs` |
| **REST API Mode** | High-Availability Dual Mode — Asynchronous execution by default. Immediate dynamic fallback to **Inline Synchronous Processing** on bus channel exceptions, returning 200 OK + items immediately. |
| **Docker** | `Dockerfile` only (multi-stage) |
| **Logging** | Structured JSON via `log/slog` |

---

## Project Layout

```
rssservice/
├── cmd/
│   └── rssservice/
│       └── main.go             # Entry point — initializes configuration and boots hot services
├── internal/
│   ├── config/
│   │   └── config.go           # Load YAML/TOML + env overrides (with Cache TTL configurations)
│   ├── server/
│   │   ├── server.go           # HTTP server setup, routing, graceful shutdown
│   │   ├── middleware/
│   │   │   ├── auth.go         # JWT ES256 validation + Redis blocklist check
│   │   │   └── logging.go      # Request/response structured logging
│   │   └── handler/
│   │       ├── parse.go        # POST /parse — attempts async enqueueing; catches channel errors for sync fallback
│   │       ├── jobs.go         # GET  /jobs/{id} — poll job status from Redis state store
│   │       └── health.go       # GET  /health
│   ├── worker/
│   │   ├── worker.go           # RabbitMQ AMQP consumer loop (rss_commands_worker subscription)
│   │   └── processor.go        # Checks Redis URL cache, runs rssreader.Parse, updates Redis jobstore, publishes results
│   ├── rabbitmq/
│   │   ├── client.go           # Channel manager with long-lived worker connections
│   │   ├── consumer.go         # AMQP 0-9-1 consumer binding rules for rss_commands
│   │   └── publisher.go        # AMQP 0-9-1 publisher — pushes to rss_results exchange
│   ├── jobstore/
│   │   └── jobstore.go         # Job state in Redis (status, metadata, TTL)
│   ├── cache/
│   │   └── urlcache.go         # In-memory Redis lookup cache for fingerprints with explicit TTL windows
│   ├── auth/
│   │   └── jwt.go              # ES256 parse/validate + blocklist check
│   └── docs/
│       └── openapi.yaml        # OpenAPI 3.1 spec
├── Dockerfile                  # Multi-stage build
├── config.yaml                 # Default configuration file
├── go.mod
├── go.sum
└── README.md
```

---

## Configuration (`config.yaml` + env overrides)

```yaml
server:
  port: 8080
  read_timeout:  15s
  write_timeout: 30s
  idle_timeout:  60s

redis:
  dsn: "redis://localhost:6379/0"
  job_ttl: 1h
  cache_ttl_seconds: 3600      # 1-hour strict explicit TTL window for parsed URL cache entities
  blocklist_prefix: "jwt:blocklist:"

rabbitmq:
  url: "amqp://guest:guest@localhost:5672/"
  exchanges:
    commands: "rss_commands"   # Direct exchange
    results: "rss_results"     # Direct exchange
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
  public_key_path: "./keys/ec_public.pem"  # ES256 public key

rssreader:
  concurrency_limit: 10
  max_body_size:     10485760  # 10 MB

log:
  level: "info"   # debug | info | warn | error
```

Every key maps to an env var override using `_`-separated uppercase paths:
`SERVER_PORT`, `REDIS_DSN`, `REDIS_CACHE_TTL_SECONDS`, `RABBITMQ_URL`, etc.

---

## Public REST API

All endpoints except `GET /health` and `GET /docs` require:
```
Authorization: Bearer <ES256-signed JWT>
```

### Endpoints

| Method | Path | Auth | Primary Mode Behavior | Fallback Mode Behavior (On Bus Error) |
|---|---|---|---|---|
| `GET` | `/health` | ✗ | Liveness check | Liveness check |
| `GET` | `/docs` | ✗ | Swagger UI | Swagger UI |
| `POST` | `/parse` | ✓ | Enqueue to RabbitMQ → Return `202 Accepted` + `job_id` | **Synchronous Parse** (checks Redis cache first) → Return `200 OK` + full item JSON inline |
| `GET` | `/jobs/{id}` | ✓ | Poll job status + results from Redis | Poll job status from Redis |

### `POST /parse` — Primary Asynchronous Response (`202 Accepted`):
```json
{ "job_id": "01J3K..." }
```

### `POST /parse` — Synchronous Fallback Response (`200 OK`):
Returned seamlessly if `POST /parse` catches a channel exception from a down message bus during execution.
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

## Caching & Message Routing Design

### Redis Transient URL Cache Strategy
To optimize network usage and avoid duplicate processing cycles, a dedicated caching tier handles URL outputs inside the core processor logic:
1. **Fingerprint Validation:** The worker converts target URLs into uniform keys (`cache:url:<md5_or_sha256_hash>`).
2. **In-Memory Verification:** Prior to establishing outbound sockets via `rssreader.Parse`, the worker checks Redis for an active cache hit.
3. **Short-Circuit Re-routing:** On a cache hit, the raw JSON bytes are deserialized directly and stuffed into the `rss_results` message payload immediately, bypassing outbound network amplification.
4. **Cache Hydration:** On a cache miss, the service completes a fresh fetch, wraps the output, and issues an atomic `SETEX` command to save the results back into Redis with an explicit `cache_ttl_seconds` window.

### RabbitMQ Topology & DLX Workflow

The architecture uses Direct Exchanges to guarantee fast routing and message safety.

```
Exchange: rss_commands (Direct, durable: true)
   └── Binds to Queue: rss_commands_worker (durable: true)
       └── Rejection Policy (DLX): Configured to use 'rss_commands_retry'

Exchange: rss_commands_retry (Direct, durable: true)
   └── Binds to Queue: rss_commands_wait_30s (durable: true)
       ├── Arguments: x-message-ttl: 30000 (30 seconds)
       └── Arguments: x-dead-letter-exchange: "rss_commands"

Exchange: rss_results (Direct, durable: true)
   └── Binds to Queue: rss_results_rails (durable: true)
```

1. **Failure Escalation:** When structural execution drops or errors out inside the Go parsing module, the message is NACKed or rejected. It routes automatically to the `rss_commands_wait_30s` queue via the retry exchange.
2. **Re-Queueing Loop:** Because the wait queue has no active consumers, messages expire naturally after 30 seconds and route back to the original `rss_commands` exchange for another ingestion attempt.
3. **Dead-Letter Drop:** If execution fails 3 consecutive times (verified via the AMQP `x-death` header), the message drops directly into the `rss_commands_failed` queue.
4. **Daily Sweep:** The `rss_commands_failed` queue undergoes an automated daily sweep routine to parse structural error trends and clear storage blockages.

---

## Worker Execution Flow

### Asynchronous Consumer Engine (`worker.go` + `processor.go`)

The API services and worker structures stay hot and listening continuously. The execution path implements an optimistic delivery pipeline:

```
worker engine initialization
  └── Establish long-lived channel connections via github.com/rabbitmq/amqp091-go
  └── Declare durable exchanges (rss_commands, rss_commands_retry, rss_results)
  └── Configure channel QoS parameters (prefetch count = 10)

continuous consumer loop
  ├── channel.Consume("rss_commands_worker", autoAck = false)
  ├── for msg := range deliveries
  │    ├── Decode incoming payload (ULID job_id + URLs array)
  │    ├── Atomically update Redis jobstore state: job:<id> -> "processing"
  │    │
  │    ├── [Processor Phase]
  │    │    ├── Calculate target URL fingerprint hashes
  │    │    ├── Query Redis URL Cache (cache:url:<hash>)
  │    │    │    ├── CACHE HIT  -> Extract serialized payload immediately
  │    │    │    └── CACHE MISS -> Call rssreader.Parse(ctx, urls)
  │    │    │                       Save fresh structures into Redis URL cache with strict TTL
  │    │    │
  │    │    ├── Serialize results to JSON
  │    │    ├── Publish message to exchange "rss_results" with key "rss_results_rails" (persistent: true)
  │    │    └── Update Redis jobstore state: job:<id> -> "done" | "failed"
  │    │
  │    └── msg.Ack(false) -> Manual acknowledgement transmitted upon success
  │
  └── On channel exception -> Init reconnection loop with exponential backoff
```

---

## Test Plan

### Unit tests

| File | Target Focus |
|---|---|
| `internal/config/config_test.go` | Ensure configuration reads YAML keys; validates default cache TTL bounds and env string parsing. |
| `internal/auth/jwt_test.go` | Validates ES256 signatures, `exp` claim limits, and `jwt:blocklist:<jti>` Redis lookups. |
| `internal/jobstore/jobstore_test.go` | Asserts `job:<id>` JSON string mapping, status alterations, and key lifetime structures inside Redis. |
| `internal/cache/urlcache_test.go` | Verifies cache hit short-circuiting, correct hash formatting, and proper initialization of explicit TTL constraints. |
| `internal/worker/processor_test.go` | Mocks `rssreader.Parse` outcomes; checks that caching behavior suppresses unnecessary network tracking and asserts correct state transitions. |
| `internal/server/handler/parse_test.go` | Asserts that a clean execution responds with a `202 Accepted` status frame. Simulates message bus interface failures to confirm it slips into an immediate synchronous fallback block, returning a `200 OK` + item arrays. |

### Integration tests (build tag: `integration`)
* Spin up containerized instances of Redis and RabbitMQ via `testcontainers-go`.
* **Full Cycle Verification:** Hit `POST /parse` ➡️ verify async delivery to the consumer queue ➡️ check Redis URL cache population ➡️ assert that polling `GET /jobs/{id}` returns the populated dataset.
* **Fallback Validation:** Stop the RabbitMQ container while keeping the Redis container alive. Issue a `POST /parse` request and assert that the endpoint catches the channel drop, executes parsing inline, and returns a `200 OK` with payload content directly.

---

## Implementation Order

1. Initialize `go.mod` and vendor dependencies (remove any SQL drivers/ORM references entirely)
2. Build out `internal/config/config.go` (incorporating `cache_ttl_seconds`)
3. Write `internal/auth/jwt.go` and verification middlewares
4. Write `internal/jobstore/jobstore.go` and `internal/cache/urlcache.go` to handle Redis states and TTL caching
5. Build out `internal/rabbitmq/publisher.go` and `consumer.go` using durable exchanges and persistent flags
6. Build out `internal/worker/processor.go` core parsing rules (Cache Lookup ➡️ Parse ➡️ Write Cache)
7. Build `internal/worker/worker.go` long-lived channel consumer loops
8. Code endpoint handlers (`parse.go`, `jobs.go`, `health.go`), embedding immediate rescue logic inside `parse.go` to support synchronous fallback routing
9. Wire routing boundaries and server components inside `internal/server/server.go`
10. Finalize the `cmd/rssservice/main.go` boot orchestration lifecycle and verify the multi-stage `Dockerfile`
