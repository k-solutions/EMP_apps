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
> **Why this error happened:** 
> In standard Rails 8 configurations, Puma or ActiveJob tries to connect to Solid Queue database tables on startup. Since our architecture completely bypasses ActiveJob, Sidekiq, and Solid Queue in favor of a direct RabbitMQ publisher and Sneakers, **Solid Queue has been entirely removed from the Gemfile and disabled in `config/application.rb`**. 
>
> This isolates the database completely from Solid Queue, preventing any "relation 'solid_queue_jobs' does not exist" ActiveRecord statement invalid crashes.

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

### Step 3: Start Development Servers
Start both the Rails API server and the Vite asset dev server using the dev wrapper:
```bash
bin/dev
```
Open `http://localhost:3000` in your browser. Use the seed credentials `user@example.com` / `password` to log in.

### Step 4: Start Sneakers Ingestion Worker
To consume parsing results published by the Go worker to RabbitMQ in the background:
```bash
env DATABASE_URL=postgres://postgres@localhost:5432/rss_frontend_development ./bin/rails sneakers:run
```

---

## 5. Running Automated Tests

Run the complete RSpec unit, request, and model test suites:
```bash
env DATABASE_URL=postgres://postgres@localhost:5432/rss_frontend_test bundle exec rspec spec/requests spec/services spec/models spec/channels spec/jobs
```

All core unit and request tests should pass cleanly with **0 failures**.

> [!IMPORTANT]
> Headless Chrome is not installed natively in the FHS profile of `flake.nix`, meaning the Selenium-dependent browser automation specs (`spec/features/*`) cannot initialize a local Chrome Webdriver. However, the core Rails models, direct Bunny publishing requests, and Sneakers ActionCable broadcast jobs are fully tested and pass successfully.
