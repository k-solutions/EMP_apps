# RSS Reader — Rails 8 + React Frontend

A premium, state-of-the-art RSS Reader frontend built with **Rails 8 (API + Session Auth)**, a **Vite-bundled React SPA** styled with **Bootstrap 5**, and a robust **RabbitMQ topic exchange integration via Sneakers** that completely bypasses background job queues (like Sidekiq/ActiveJob) on submission.

---

## 1. Architecture Overview

The application utilizes a direct-publish, asynchronous architecture designed for maximum performance, low latency, and absolute simplicity.

```mermaid
graph TD
  ReactSPA[React SPA Frontend] <-->|WebSocket: ActionCable| RailsServer[Rails 8 API Server]
  ReactSPA <-->|HTTP JSON API| RailsServer

  subgraph Messaging Pipeline (Asynchronous Topic Exchange)
    RailsServer -->|RabbitmqPublisher| DirectCommands{Exchange: rss}
    DirectCommands -->|rss.commands.*| GoWorker[Go Worker]
    GoWorker -->|rss.results.*| DirectResults{Exchange: rss}
    DirectResults --> ProcessFeedJob[ProcessFeedResultJob]
    ProcessFeedJob -.->|ActionCable| RailsServer
  end
```

### Flow Breakdown
1. **Direct Command Publishing**:
   - Upon feed submission at `POST /api/v1/feeds`, the controller immediately instantiates `RabbitmqPublisher` (using Bunny) and publishes the command payload directly to the `"rss"` topic exchange under the routing key `rss.commands.<job_id>`.
   - The `FeedRequest` is updated to `processing` on successful publish. The controller returns a `202 Accepted` status immediately.
2. **Go Parsing Service**:
   - The Go backend `rssservice` consumes the command via its queue, parses the RSS feeds (caching results in Redis), and publishes the result payload back to the `"rss"` exchange under routing key `rss.results.rails`.
3. **Sneakers Result Ingestion**:
   - The Sneakers worker `ProcessFeedResultJob` runs in a background process, consumes results from the `rss.results` queue, persists parsed items into the PostgreSQL database, and broadcasts updates via ActionCable to the React client.

---

## 2. Fixing "relation 'solid_queue_jobs' does not exist" Error

> [!NOTE]
All Rails and React functionality must be coverd by test 
---

## 3. Environment Variables

Configure these in your environment or a `.env` file at the root of `rss_frontend/`:

| Variable | Description | Default |
|---|---|---|
| `DATABASE_URL` | PostgreSQL connection URL | `postgres://postgres@localhost:5432/rss_frontend_development` |
| `REDIS_URL` | Redis connection URL | `redis://localhost:6379/0` |
| `RABBITMQ_URL` | RabbitMQ connection URL | `amqp://guest:guest@localhost:5672/` |

---

## 4. Getting Started & Development Workflow

We recommend using the system-wide Nix development sandbox environment (`nix develop ..` at the workspace root) to execute database preparations, npm packages, and workers natively:

### Step 1: Database Initialization & Seeding
Prepare the development database, run migrations, and load the seed data:
```bash
# Setup the development and test databases
env DATABASE_URL=postgres://postgres@localhost:5432/rss_frontend_development ./bin/rails db:prepare
env RAILS_ENV=test DATABASE_URL=postgres://postgres@localhost:5432/rss_frontend_test ./bin/rails db:prepare

# Load initial development seed data
env DATABASE_URL=postgres://postgres@localhost:5432/rss_frontend_development ./bin/rails db:seed
```

> [!TIP]
> **What the seed data provides:**
> - A default user account (`user@example.com` / `password`)
> - Initial sample RSS feed requests and parsed RSS items to populate the React UI instantly upon first login.

### Step 2: Install NPM Packages
```bash
npm install
```

### Step 3: Start Development Environment (Unified Foreman Workflow)
You can start all Rails servers, Vite asset compilers, Sneakers ingestion workers, and Solid Queue background workers simultaneously using the unified dev wrapper (orchestrated via the `foreman` gem):
```bash
bin/dev
```
This single command automatically spins up and monitors:
1. **Rails API Server** (Port `3000`)
2. **Vite Asset Compiler** (Port `5173`)
3. **Sneakers Result Ingestion Worker** (Event-driven queue subscriber)
4. **Solid Queue Supervisor** (Background task executor and retry orchestrator)

Open `http://localhost:3000` in your browser. Use the seed credentials `user@example.com` / `password` to log in.

---

## 5. Running with Docker Compose (Multi-Container Stack)

If you prefer to run the entire stack (Rails API, precompiled React SPA, Go `rss_service`, PostgreSQL, Redis, and RabbitMQ) in a single unified multi-container Docker environment, you can use the configured `docker-compose.yml` file.

### Step 1: Build the Docker Images
Compile the Docker images for all services (this will precompile the frontend React SPA assets inside the Rails production-optimized container):
```bash
docker compose build
```

### Step 2: Database Initialization & Seeding inside Docker
Run the ActiveRecord migrations and database seed command to initialize PostgreSQL and load the default login credentials and sample RSS items inside the active containers:
```bash
docker compose run --rm rails bundle exec rails db:prepare db:seed
```

### Step 3: Spin Up all Services
Start all containers:
```bash
# Start all services in the foreground
docker compose up

# Or run in detached background mode
docker compose up -d
```
This spins up and exposes:
- **PostgreSQL (`db`)** on port `5432`
- **Redis (`redis`)** on port `6379`
- **RabbitMQ (`rabbitmq`)** on port `5672` (Management UI available on `http://localhost:15672`)
- **Go RSS Parser Service (`rss_service`)** on port `8080`
- **Rails API + React Server (`rails`)** on port `3000` (optimized production assets served statically)
- **Sneakers AMQP Worker (`sneakers`)** consuming parsing results from the RabbitMQ pipeline

Open `http://localhost:3000` in your browser and log in with the preloaded credentials:
- **Email**: `user@example.com`
- **Password**: `password`

### Step 4: Tear Down
To halt and remove all running containers, networks, and services:
```bash
docker compose down
```

---

## 6. Running Automated Tests

Run the complete RSpec unit, request, and model test suites:
```bash
env DATABASE_URL=postgres://postgres@localhost:5432/rss_frontend_test bundle exec rspec spec/requests spec/services spec/models spec/channels spec/jobs
```

All core unit and request tests should pass cleanly with **0 failures**.

