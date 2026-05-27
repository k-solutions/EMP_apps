# High-Concurrency RSS Reader System

A premium, state-of-the-art distributed RSS/Atom feed parsing platform built for maximum performance, cryptographic security, and extreme resilience. 

Decoupled via a durable **RabbitMQ Direct Exchange** bus, optimized via a fast **Redis Transient URL Caching** tier, and orchestrated by a **Rails 8 + React SPA** and a high-performance **Go rssservice**, the system guarantees high availability through a **High-Availability Dual Mode** with automatic background fallback routing.

---

## 1. System Overview & DECUPLED Architecture

The application is split into two specialized services, keeping storage and execution cleanly separated:

```mermaid
graph TD
  ReactSPA[React SPA Frontend] <-->|WebSocket: ActionCable| RailsServer[Rails 8 API Server]
  ReactSPA <-->|HTTP JSON API| RailsServer

  subgraph Full Mode Path (Asynchronous)
    RailsServer -.->|1. ActiveJob / Solid Queue| PublishFeedJob[PublishFeedJob]
    PublishFeedJob -->|2. routing_key: rss_commands_worker| CommandsExchange{rss_commands exchange<br/>Direct, Durable}
    CommandsExchange -->|3. Durable Command Queue| CommandsQueue[rss_commands_worker Queue]
    CommandsQueue -->|4. AMQP Ingestion| GoWorker[Go worker processor]
    
    GoWorker -->|5. Parsed Result| ResultsExchange{rss_results exchange<br/>Direct, Durable}
    ResultsExchange -->|6. Routing: rss_results_rails| ResultsQueue[rss_results_rails Queue]
    ResultsQueue -->|7. Sneakers Ingestion| ProcessFeedJob[ProcessFeedResultJob]
    ProcessFeedJob -.->|8. Database Save & Broadcast| RailsServer
  end

  subgraph Fallback Mode Path (Synchronous REST)
    PublishFeedJob ==>|Broker Exception caught / ES256 JWT Auth| GoParseAPI[Go REST API: POST /parse]
    GoParseAPI ==>|Direct Synchronous Parse| GoWorker
  end

  GoWorker <-->|Uniform cache lookup with strict TTL| RedisCache[(Redis Cache & State)]
  RailsServer <--->|User Profiles, Configs, Solid Queue| PostgreSQL[(PostgreSQL Database)]
```

### Decoupled Components & Responsibilities

| Component | Stack | Responsibility |
| :--- | :--- | :--- |
| **Rails Frontend** | Ruby 3.3 (Rails 8) | Persistent datastore orchestration (PostgreSQL), Devise session management, ActiveJob/Solid Queue scheduling, ActionCable WebSocket streaming, and a high-fidelity Vite-bundled React SPA frontend. |
| **Go Service** | Go 1.26 | Pure parser logic, consuming command payloads via RabbitMQ or responding synchronously via REST `POST /parse`. Protects external endpoints using stateful ES256 JWT checks. |
| **RabbitMQ** | AMQP 0-9-1 | **Primary Message Bus:** Decouples the architecture using durable, disk-persistent Direct Exchanges (`rss_commands`, `rss_results`). |
| **Redis** | Key-Value Store | **Job State & Uniform Cache:** Caches parsed URL fingerprints (`cache:url:<hash>`) under strict TTL windows and manages `job:<id>` execution states. |
| **PostgreSQL** | Relational DB | **Relational Datastore:** Stores structural user profiles, feed requests, persisted feed items, and Solid Queue job metadata. |

---

## 2. High-Availability Dual Mode

The system implements a double-layered optimistic delivery pipeline ensuring zero-downtime parsing even during infrastructure or broker outages:

### A. Full Mode (Asynchronous Path)
1. **Command:** The Rails controller enqueues `PublishFeedJob` to Solid Queue.
2. **Publish:** `PublishFeedJob` publishes a command message to `rss_commands` (Direct, Durable) exchange using routing key `rss_commands_worker` and `persistent: true`.
3. **Ingest:** The Go worker consumes from `rss_commands_worker` queue, checks the Redis URL cache for fingerprints, fetches misses, hydrates the Redis cache, and writes job status updates.
4. **Result:** The Go worker publishes finalized results to `rss_results` (Direct, Durable) exchange using routing key `rss_results_rails`.
5. **Persist:** A Rails Sneakers worker (`ProcessFeedResultJob`) consumes from the `rss_results_rails` queue, updates database states, and broadcasts updates via ActionCable to the React frontend in real time.

### B. Fallback Mode (Synchronous REST Path)
1. **Detection:** Bypasses manual ping-checks. `PublishFeedJob` catches Bunny broker connection or channel exceptions inline during active service loops.
2. **REST Invocation:** It automatically switches to Fallback Mode, cryptographically signs a local ES256 JWT token using `JwtService`, and makes a synchronous `POST /parse` REST API request directly to the Go service.
3. **Parse & Return:** The Go service validates the signature, performs parsing synchronously (respecting the Redis cache layer if available), and returns a `200 OK` with full results.
4. **Save & Broadcast:** `PublishFeedJob` saves results directly into PostgreSQL and broadcasts updates to the React client via ActionCable, completely bypassing the RabbitMQ layer.

---

## 3. Project Directory Structure

```
emarchantPay/
├── ARCHITECTURE.md          # Target integration architecture specification
├── README.md                # System-Wide Root README
├── flake.nix                # Isolated Nix development sandbox configuration
├── rss_backend/
│   ├── go.work              # Go Workspace file
│   └── rssservice/          # Go worker and REST API server service
│       ├── cmd/             # Entrypoints
│       ├── internal/        # Packages (server, rabbitmq, cache, jobstore)
│       └── GEMINI.md        # Go Service detailed implementation plan
└── rss_frontend/            # Rails 8 + Vite + React SPA
    ├── app/
    │   ├── controllers/     # API controller endpoints (Feeds, Items)
    │   ├── jobs/            # PublishFeedJob (Solid Queue) & ProcessFeedResultJob (Sneakers)
    │   ├── services/        # RabbitmqPublisher & JwtService
    │   └── javascript/      # React SPA components (Vite entrypoint)
    ├── config/              # Rails application & queue configs
    ├── keys/                # Cryptographic ES256 key pairs for JWT signing
    └── GEMINI.md            # Rails Frontend detailed implementation plan
```

---

## 4. Getting Started (Nix Sandbox Environment)

The workspace is equipped with a **Nix FHS Sandbox** (`flake.nix`) providing fully configured developer paths, databases, and message brokers with zero host-level configuration.

### 1. Fire up the Development Container
Enter the Nix sandbox shell. Nix automatically installs Go 1.26, Ruby 3.3, and boots local PostgreSQL, Redis, and RabbitMQ servers:
```bash
nix develop
```

### 2. Configure Environment Keys
Generates the cryptographically secure P-256 EC Key pair inside the Go service and places the matching private key in the Rails frontend keys directory:
```bash
# In Go Service keys folder
openssl ecparam -name prime256v1 -genkey -noout -out rss_backend/rssservice/keys/ec_private.pem
openssl ec -in rss_backend/rssservice/keys/ec_private.pem -pubout -out rss_backend/rssservice/keys/ec_public.pem

# Sync private key to Rails frontend
cp rss_backend/rssservice/keys/ec_private.pem rss_frontend/keys/ec_private.pem
```

### 3. Setup Rails Datastores & Install Packages
```bash
cd rss_frontend
bundle install
npm install
bundle exec rails db:prepare
```

### 4. Boot the Servers
To run the full stack locally:
- **Go service:** `cd rss_backend/rssservice && go run ./cmd/rssservice`
- **Rails web & Vite server:** `cd rss_frontend && bin/dev`
- **Sneakers consumer:** `cd rss_frontend && bundle exec rake sneakers:run`

---

## 5. Running Automated Tests

Both components are backed by extremely thorough unit and integration test suites:

### Rails Frontend RSpec Tests
Runs unit tests, background job publishers, Sneakers manual acks, Devise endpoints, and the fallback REST path:
```bash
cd rss_frontend
bundle exec rspec spec/services spec/jobs spec/requests spec/models
```

### Go Service Tests
Runs authentications, JWT validations, Redis URL caching, and endpoint handlers:
```bash
cd rss_backend/rssservice
go test -v ./...
```