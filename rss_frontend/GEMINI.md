# RSS Reader Rails Frontend — Implementation Plan

## All Decisions

| Concern | Decision |
|---|---|
| Rails version | Rails 8 (latest) |
| View engine | Slim (`slim-rails`) |
| CSS framework | Bootstrap 5 — npm, imported via Vite (React only) |
| React bundler | Vite + React (`vite-ruby`) |
| Rails + React integration | React SPA consuming Rails JSON API |
| Inter-service bus | RabbitMQ — Direct exchanges, AMQP 0-9-1 (Primary asynchronous execution path) |
| RabbitMQ properties | Durable Exchanges (`rss_commands`, `rss_results`) + Durable Queues (`rss_commands_worker`, `rss_results_rails`) + Explicit message durability (`persistent: true`) + Manual Acks |
| RabbitMQ consumer | Sneakers gem (Sidekiq-style AMQP worker) |
| Background jobs | ActiveJob using Solid Queue (for orchestrating `PublishFeedJob`) + Sneakers (for consuming RabbitMQ results) |
| Job trigger | Controller enqueues `PublishFeedJob` asynchronously, which attempts to publish to RabbitMQ, falling back to synchronous HTTP via Go `POST /parse` on exception |
| Rails user auth | Devise |
| Database | PostgreSQL |
| RSS item persistence | Store parsed items in Rails DB (cache) |
| Rails tests | RSpec |
| React tests | Jest + React Testing Library |
| Docker | Docker Compose (Rails + Redis + RabbitMQ + rss_service) |
| Capybara driver | Selenium (Chrome headless) |
| Integration stack | Full Docker Compose (real Redis + RabbitMQ + rss_service) |
| DB cleaning | DatabaseCleaner (truncation strategy) |
| Capybara spec types | Feature specs + request specs |
| Test data | Rails fixtures |

---

## Project Layout

```
rss_frontend/
├── app/
│   ├── views/
│   │   ├── layouts/
│   │   │   └── application.html.slim   # Shell page — mounts React SPA
│   │   └── devise/                     # Slim-rendered auth views (fallback only)
│   │       ├── sessions/
│   │       │   └── new.html.slim       # Login form (if JS disabled)
│   │       └── shared/
│   │           └── _error_messages.html.slim
│   ├── channels/
│   │   └── feed_channel.rb          # ActionCable channel — streams results to React
│   ├── controllers/
│   │   ├── api/
│   │   │   ├── v1/
│   │   │   │   ├── feeds_controller.rb     # POST /api/v1/feeds — submit URLs
│   │   │   │   ├── feed_items_controller.rb # GET /api/v1/feed_items — cached results
│   │   │   │   └── health_controller.rb    # GET /api/v1/health — status
│   │   │   └── base_controller.rb          # Auth + JSON response helpers
│   │   └── application_controller.rb
│   ├── models/
│   │   ├── user.rb                  # Devise user
│   │   ├── feed_request.rb          # Tracks a parse job (job_id, status, urls)
│   │   └── feed_item.rb             # Cached RSS item (belongs_to feed_request)
│   ├── services/
│   │   ├── rabbitmq_publisher.rb       # AMQP 0-9-1 publish to rss_commands direct exchange
│   │   └── jwt_service.rb              # Generates ES256 signed JWT tokens for fallback HTTP requests
│   ├── jobs/
│   │   ├── publish_feed_job.rb         # ActiveJob (Solid Queue) enqueuing job with dynamic REST fallback
│   │   └── process_feed_result_job.rb  # Sneakers worker — consumes rss_results_rails from RabbitMQ
│   ├── javascript/
│   │   ├── entrypoints/
│   │   │   └── application.jsx     # Vite entrypoint — mounts React SPA
│   │   ├── components/
│   │   │   ├── App.jsx             # Router root
│   │   │   ├── auth/
│   │   │   │   ├── LoginForm.jsx
│   │   │   │   └── LoginForm.test.jsx
│   │   │   ├── feeds/
│   │   │   │   ├── FeedForm.jsx        # URL input + submit
│   │   │   │   ├── FeedForm.test.jsx
│   │   │   │   ├── FeedList.jsx        # Displays RssItems
│   │   │   │   ├── FeedList.test.jsx
│   │   │   │   ├── FeedItem.jsx        # Single item card
│   │   │   │   └── FeedItem.test.jsx
│   │   │   └── shared/
│   │   │       ├── StatusBadge.jsx     # pending | processing | done | failed
│   │   │       └── ErrorBanner.jsx
│   │   ├── hooks/
│   │   │   ├── useFeedChannel.js   # ActionCable subscription hook
│   │   │   └── useFeedChannel.test.js
│   │   ├── api/
│   │   │   └── client.js           # fetch wrapper (credentials: include for cookie)
│   │   └── cable.js                # ActionCable consumer setup
├── config/
│   ├── initializers/
│   │   └── rabbitmq.rb             # Verifies RabbitMQ connection on boot, exits on failure
│   ├── cable.yml                   # ActionCable → Redis adapter
│   ├── database.yml                # PostgreSQL config
│   └── routes.rb
├── db/
│   └── migrate/
│       ├── XXXXXX_devise_create_users.rb
│       ├── XXXXXX_create_feed_requests.rb
│       └── XXXXXX_create_feed_items.rb
├── spec/
│   ├── models/
│   │   ├── feed_request_spec.rb
│   │   └── feed_item_spec.rb
│   ├── services/
│   │   └── rabbitmq_publisher_spec.rb
│   ├── controllers/
│   │   └── api/v1/
│   │       ├── feeds_controller_spec.rb
│   │       └── feed_items_controller_spec.rb
│   ├── channels/
│   │   └── feed_channel_spec.rb
│   ├── jobs/
│   │   └── process_feed_result_job_spec.rb # Sneakers worker spec
│   ├── features/                        # Capybara feature specs
│   │   ├── authentication_spec.rb       # Login / logout flows
│   │   ├── feed_submission_spec.rb      # Feed submission flow
│   │   └── feed_items_display_spec.rb   # Cached items rendering
│   ├── requests/                        # Capybara request specs (API layer)
│   │   ├── feeds_request_spec.rb
│   │   └── feed_items_request_spec.rb
│   ├── support/
│   │   ├── capybara.rb                  # Driver config — Selenium Chrome headless
│   │   ├── database_cleaner.rb          # Truncation strategy
│   │   └── fixtures.rb                  # Fixture helper shortcuts
│   ├── fixtures/
│   │   ├── users.yml
│   │   ├── feed_requests.yml
│   │   └── feed_items.yml
│   └── rails_helper.rb
├── Dockerfile
├── docker-compose.yml
├── vite.config.ts
├── jest.config.js
├── package.json
├── Gemfile
└── README.md
```

---

## Database Schema

### `users` (Devise)
| Column | Type | Notes |
|---|---|---|
| `id` | bigint PK | |
| `email` | string | unique, not null |
| `encrypted_password` | string | |
| `jwt_jti` | string | for JWT blocklist awareness |
| `created_at` / `updated_at` | datetime | |

### `feed_requests`
| Column | Type | Notes |
|---|---|---|
| `id` | bigint PK | |
| `user_id` | bigint FK | belongs_to user |
| `job_id` | string | ULID from rss_service |
| `urls` | text[] | PostgreSQL array |
| `status` | string | `pending \| processing \| done \| failed` |
| `created_at` / `updated_at` | datetime | |

### `feed_items`
| Column | Type | Notes |
|---|---|---|
| `id` | bigint PK | |
| `feed_request_id` | bigint FK | belongs_to feed_request |
| `title` | string | |
| `source` | string | |
| `source_url` | string | |
| `link` | string | unique index per feed_request |
| `publish_date` | date | date only — matches `rssreader.Date` |
| `description` | text | |
| `created_at` / `updated_at` | datetime | |

---

## Rails JSON API

All endpoints under `/api/v1/`. Auth via Devise session cookie.

### Endpoints

| Method | Path | Auth | Description |
|---|---|---|---|
| `POST` | `/api/v1/users/sign_in` | ✗ | Devise login → sets session cookie |
| `DELETE` | `/api/v1/users/sign_out` | ✓ | Devise logout → clears session cookie |
| `POST` | `/api/v1/feeds` | ✓ | Submit URLs → enqueue `PublishFeedJob` → return `feed_request` |
| `GET` | `/api/v1/feed_items` | ✓ | List cached items for current user |
| `GET` | `/api/v1/health` | ✗ | Rails + RabbitMQ status |

### `POST /api/v1/feeds`

**Request:**
```json
{ "urls": ["https://feeds.bbci.co.uk/news/rss.xml"] }
```

**Response `202`:**
```json
{
  "feed_request_id": 42,
  "job_id": "01J3K...",
  "status": "pending"
}
```

**Feed submission flow:**
1. Rails generates a ULID `job_id` — it owns the ID.
2. `FeedRequest` created with `status: pending`, `job_id` stored.
3. Controller enqueues `PublishFeedJob.perform_later(feed_request)` asynchronously and returns `202 Accepted` immediately.
4. **Asynchronous Path (Full Mode):**
   - `PublishFeedJob` runs inside Solid Queue.
   - It publishes command to `rss_commands` (Direct, Durable) exchange using routing key `rss_commands_worker` and `persistent: true`.
   - `FeedRequest` status is updated to `processing` on successful publish.
   - Go worker consumes from `rss_commands_worker` queue, parses feeds (utilizing the Redis URL cache), and publishes results to `rss_results` (Direct, Durable) exchange with routing key `rss_results_rails`.
   - Sneakers `ProcessFeedResultJob` worker consumes from `rss_results_rails`, inserts items into PostgreSQL, updates `FeedRequest` to `done` (or `failed`), and broadcasts via ActionCable.
5. **Fallback Path (Fallback Mode):**
   - If `PublishFeedJob` catches a runtime connection error from the message bus, it enters fallback mode.
   - It signs an ES256 JWT token using `JwtService` and sends a synchronous `POST /parse` REST request directly to the Go worker.
   - The Go worker parses the feeds synchronously (respecting the Redis cache layer if available) and returns the JSON payload inline with `200 OK`.
   - `PublishFeedJob` saves the returned items to PostgreSQL, updates `FeedRequest` status, and broadcasts the results to the client via ActionCable, completely bypassing RabbitMQ.

---

## RabbitMQ Connection Verification (`config/initializers/rabbitmq.rb`)

During application boot (both Rails web server and Sneakers workers), we verify that RabbitMQ is reachable. If it is not present, we output a clear error message and exit immediately.

```ruby
# config/initializers/rabbitmq.rb
begin
  conn = Bunny.new(ENV.fetch("RABBITMQ_URL", "amqp://guest:guest@localhost:5672"))
  conn.start
  conn.close
rescue => e
  warn "========================================================="
  warn "FATAL: RabbitMQ is not available. Exiting."
  warn "Error details: #{e.message}"
  warn "========================================================="
  exit 1
end
```

Called during initializers execution on application start.

---

## Background Jobs — ActiveJob & Sneakers

Background worker pools orchestrate the asynchronous operations of the app: `PublishFeedJob` (via Solid Queue ActiveJob) and `ProcessFeedResultJob` (via Sneakers).

### `PublishFeedJob` (`app/jobs/publish_feed_job.rb`)

This ActiveJob runs asynchronously using the Solid Queue backend. It attempts to publish a parsing command to the durable `rss_commands` Direct exchange. If RabbitMQ is down, it catches the runtime error, falls back to dynamic REST processing via Go, saves results to Postgres, and broadcasts the status via ActionCable.

```ruby
class PublishFeedJob < ApplicationJob
  queue_as :default

  def perform(feed_request)
    publisher = RabbitmqPublisher.new
    begin
      publisher.publish(
        routing_key: "rss_commands_worker",
        payload:     { job_id: feed_request.job_id, urls: feed_request.urls }.to_json
      )
      feed_request.update!(status: "processing")
    rescue => e
      Rails.logger.warn "RabbitMQ publishing failed. Dropping back to Fallback Mode: #{e.message}"
      execute_fallback(feed_request)
    ensure
      publisher.close rescue nil
    end
  end

  private

  def execute_fallback(feed_request)
    # Generate ES256 JWT
    token = JwtService.generate_token(user_id: feed_request.user_id)

    # Invoke Go REST API synchronously
    uri = URI("#{ENV.fetch('RSS_SERVICE_URL', 'http://localhost:8080')}/parse")
    req = Net::HTTP::Post.new(uri)
    req["Authorization"] = "Bearer #{token}"
    req["Content-Type"] = "application/json"
    req.body = { urls: feed_request.urls }.to_json

    res = Net::HTTP.start(uri.hostname, uri.port) do |http|
      http.request(req)
    end

    if res.code == "200"
      data = JSON.parse(res.body)
      
      # Since Go service returns a new job_id in /parse, we update the request to match
      feed_request.update!(job_id: data["job_id"], status: data["status"])

      items = data["items"] || []
      errors = data["errors"] || []

      if items.any?
        feed_items_data = items.map do |item|
          pub_date = item["publish_date"] || item["Date"]
          {
            feed_request_id: feed_request.id,
            title:           item["title"],
            source:          item["source"],
            source_url:      item["source_url"],
            link:            item["link"],
            publish_date:    pub_date,
            description:     item["description"],
            created_at:      Time.current,
            updated_at:      Time.current
          }
        end
        FeedItem.insert_all(feed_items_data)
      end

      ActionCable.server.broadcast("feed_#{feed_request.user_id}", {
        feed_request_id: feed_request.id,
        status:          data["status"],
        items:           items,
        errors:          errors
      })
    else
      raise "Go RSS parser fallback failed with HTTP #{res.code}"
    end
  rescue => e
    Rails.logger.error "Fallback path execution failed for FeedRequest ##{feed_request.id}: #{e.message}"
    feed_request.update!(status: "failed")
    ActionCable.server.broadcast("feed_#{feed_request.user_id}", {
      feed_request_id: feed_request.id,
      status:          "failed",
      items:           [],
      errors:          [e.message]
    })
  end
end
```

### `ProcessFeedResultJob` (`app/jobs/process_feed_result_job.rb`)

Sneakers worker — binds to the durable `rss_results` Direct exchange and consumes from the durable `rss_results_rails` queue. It requires manual acknowledgement.

```ruby
class ProcessFeedResultJob
  include Sneakers::Worker

  from_queue "rss_results_rails",
    exchange:      "rss_results",
    exchange_type: :direct,
    routing_key:   "rss_results_rails",
    durable:       true,
    ack:           :manual

  def work_with_params(payload, delivery_info, _metadata)
    data   = JSON.parse(payload)
    job_id = data["job_id"]

    request = FeedRequest.find_by(job_id: job_id)
    unless request
      Rails.logger.warn "unknown_job_id: #{job_id}"
      return ack!
    end

    # Idempotency guard — prevents double processing if message is delivered twice
    return ack! if request.status == "done" || request.status == "failed"

    # Optimistic lock — prevents duplicate processing
    updated = FeedRequest.where(job_id: job_id, status: ["pending", "processing"])
                         .update_all(status: data["status"])
    return ack! if updated == 0

    items  = data["items"]  || []
    errors = data["errors"] || []

    if items.any?
      feed_items_data = items.map do |item|
        pub_date = item["publish_date"] || item["Date"]
        {
          feed_request_id: request.id,
          title:           item["title"],
          source:          item["source"],
          source_url:      item["source_url"],
          link:            item["link"],
          publish_date:    pub_date,
          description:     item["description"],
          created_at:      Time.current,
          updated_at:      Time.current
        }
      end
      FeedItem.insert_all(feed_items_data)
    end

    ActionCable.server.broadcast("feed_#{request.user_id}", {
      feed_request_id: request.id,
      status:          data["status"],
      items:           items,
      errors:          errors
    })

    ack!
  rescue => e
    Rails.logger.error "process_feed_result_error for job_id #{job_id}: #{e.message}"
    reject! # requeue for retry
  end
end
```

### RabbitMQ Publisher (`app/services/rabbitmq_publisher.rb`)

```ruby
class RabbitmqPublisher
  EXCHANGE = "rss_commands".freeze

  def initialize
    @conn    = Bunny.new(ENV.fetch("RABBITMQ_URL", "amqp://guest:guest@localhost:5672"))
    @conn.start
    @channel = @conn.create_channel
    @exchange = @channel.direct(EXCHANGE, durable: true)
  end

  def publish(routing_key: "rss_commands_worker", payload:)
    @exchange.publish(
      payload,
      routing_key: routing_key,
      persistent:  true   # persistent messages for durability
    )
  end

  def close
    @conn.close
  rescue => e
    Rails.logger.warn("Failed to close RabbitMQ connection: #{e.message}")
  end
end
```

### `JwtService` (`app/services/jwt_service.rb`)

```ruby
class JwtService
  def self.generate_token(user_id:)
    payload = {
      sub: user_id,
      exp: (Time.current + 5.minutes).to_i
    }
    # Read ES256 EC Private Key (matching backend ec_public.pem pair)
    private_key_path = ENV.fetch("JWT_PRIVATE_KEY_PATH", Rails.root.join("keys", "ec_private.pem"))
    private_key_pem = File.read(private_key_path)
    private_key = OpenSSL::PKey::EC.new(private_key_pem)

    JWT.encode(payload, private_key, "ES256")
  end
end
```

### Sneakers configuration (`config/sneakers.yml`)

```yaml
amqp: <%= ENV.fetch("RABBITMQ_URL", "amqp://guest:guest@localhost:5672") %>
vhost: /
heartbeat: 30
workers: 2
threads: 2
log: Rails.logger
```

---

## Messaging Design (Rails side)

### Architecture overview

```
  POST /api/v1/feeds (Controller)
         │
         │ (asynchronously enqueues)
         ▼
  PublishFeedJob (ActiveJob / Solid Queue)
         │
         ├─── (Full Mode Path) ───►  PUBLISH to Direct Exchange 'rss_commands' ──► Go Consumer Queue 'rss_commands_worker'
         │                                                                                  │
         │                                                                           rssreader.Parse
         │                                                                                  │
         │                                                                         PUBLISH to Direct Exchange 'rss_results'
         │                                                                                  │
         │                                                                                  ▼
         │                                                                         ProcessFeedResultJob (Sneakers)
         │                                                                                  │
         │                                                                         FeedItem.insert_all
         │                                                                         ActionCable → React
         │
         └─── (Fallback Mode Path) ──► HTTP POST /parse (JWT ES256 Auth)
                                                │
                                          Go Sync Parse
                                                │
                                          FeedItem.insert_all
                                          ActionCable → React
```

### RabbitMQ topology

```
Exchange: rss_commands (direct, durable: true)
  └── Routing Key: rss_commands_worker  →  Queue: rss_commands_worker (consumed by Go worker)

Exchange: rss_results (direct, durable: true)
  └── Routing Key: rss_results_rails   →  Queue: rss_results_rails (consumed by Sneakers worker)
```

### `rss_commands_worker` message payload

```json
{ "job_id": "01J3K...", "urls": ["https://feeds.bbci.co.uk/news/rss.xml"] }
```

### `rss_results_rails` message payload

```json
{
  "job_id":  "01J3K...",
  "status":  "done",
  "items":   [{ "title": "...", "source": "...", "source_url": "...",
                "link": "...", "publish_date": "2026-05-23", "description": "..." }],
  "errors":  []
}
```

### ActionCable — frontend push only (`app/channels/feed_channel.rb`)

ActionCable's role remains narrowly scoped: **WebSocket transport to React**. It is called by both the Sneakers worker (Full Mode) and the ActiveJob worker (Fallback Mode) upon completing their respective processing workflows.

```ruby
class FeedChannel < ApplicationCable::Channel
  def subscribed
    stream_from "feed_#{current_user.id}"
  end
end
```

`config/cable.yml` uses Rails-local Redis:

```yaml
production: &default
  adapter: redis
  url: <%= ENV.fetch("REDIS_URL", "redis://localhost:6379/0") %>
development:
  <<: *default
test:
  adapter: test
```

### Shared contract table

| Interface / Exchange | Written by | Read by | Type / Routing Key |
|---|---|---|---|
| `rss_commands` (RabbitMQ) | `PublishFeedJob` (Rails) | Go worker (AMQP consumer) | Direct / `rss_commands_worker` |
| `rss_results` (RabbitMQ) | Go worker | `ProcessFeedResultJob` (Sneakers) | Direct / `rss_results_rails` |
| `/parse` (HTTP REST fallback) | `PublishFeedJob` (Rails) | Go REST server | ES256 JWT Authenticated |

### `useFeedChannel` hook (React — unchanged)

```js
export function useFeedChannel(onMessage) {
  useEffect(() => {
    const sub = consumer.subscriptions.create("FeedChannel", {
      received(data) { onMessage(data) }
    })
    return () => sub.unsubscribe()
  }, [])
}
```

---

## React SPA Structure

```
App.jsx
  └── <AuthGuard>          — redirect to /login if no session
        └── <FeedForm>     — URL inputs + submit button
              └── POST /api/v1/feeds
                    └── subscribe ActionCable → await broadcast
  └── <FeedList>           — renders FeedItem cards
        └── sorted by publish_date desc (matches rssreader sort order)
```

### State flow

```
User submits URLs
  → POST /api/v1/feeds → 202 { feed_request_id, status: "processing" }
  → FeedList shows StatusBadge "processing"
  → Go worker parses → PUBLISH rss.results.<job_id>
  → Sneakers ProcessFeedResultJob → ActionCable broadcast
  → FeedList updates (status: "done")
```

---

## Slim View Engine

`slim-rails` replaces ERB for all server-rendered views. Slim is used in two places only:

**`app/views/layouts/application.html.slim`** — the single shell page Rails serves for every route. React mounts here.

```slim
doctype html
html lang="en"
  head
    meta charset="utf-8"
    meta name="viewport" content="width=device-width, initial-scale=1"
    title RSS Reader
    = vite_client_tag
    = vite_javascript_tag "application.jsx"
  body
    #root
    / React SPA mounts into #root
```

**`app/views/devise/sessions/new.html.slim`** — fallback login page rendered server-side if JavaScript is disabled.

```slim
h2 Sign in
= form_for resource, as: resource_name, url: session_path(resource_name) do |f|
  .mb-3
    = f.label :email, class: "form-label"
    = f.email_field :email, class: "form-control", autofocus: true
  .mb-3
    = f.label :password, class: "form-label"
    = f.password_field :password, class: "form-control"
  = f.submit "Sign in", class: "btn btn-primary"
```

---

## Bootstrap 5

Imported via npm into the Vite bundle — available to all React components.

**`app/javascript/entrypoints/application.jsx`**

```jsx
import "bootstrap/dist/css/bootstrap.min.css"
import "bootstrap/dist/js/bootstrap.bundle.min.js"
import React from "react"
import { createRoot } from "react-dom/client"
import App from "../components/App"

createRoot(document.getElementById("root")).render(<App />)
```

### Component → Bootstrap class mapping

| Component | Bootstrap classes used |
|---|---|
| `LoginForm` | `form-control`, `btn btn-primary`, `alert alert-danger` |
| `FeedForm` | `form-control`, `btn btn-primary`, `btn btn-outline-secondary`, `input-group` |
| `FeedList` | `container`, `row`, `col` |
| `FeedItem` | `card`, `card-body`, `card-title`, `card-text`, `card-footer` |
| `StatusBadge` | `badge bg-warning` (pending), `badge bg-info` (processing), `badge bg-success` (done), `badge bg-danger` (failed) |
| `ErrorBanner` | `alert alert-warning alert-dismissible` |

### `StatusBadge.jsx` example

```jsx
const STATUS_CLASS = {
  pending:    "bg-warning text-dark",
  processing: "bg-info text-dark",
  done:       "bg-success",
  failed:     "bg-danger",
}

export function StatusBadge({ status }) {
  return (
    <span
      className={`badge ${STATUS_CLASS[status] ?? "bg-secondary"}`}
      data-status={status}
    >
      {status}
    </span>
  )
}
```

---

## Test Plan

### RSpec — Rails

| File | Tests |
|---|---|
| `spec/models/feed_request_spec.rb` | Validations; status transitions; associations |
| `spec/models/feed_item_spec.rb` | Validations; uniqueness of link per feed_request |
| `spec/services/rabbitmq_publisher_spec.rb` | AMQP publish called with correct routing key + payload; connection error handled |
| `spec/jobs/process_feed_result_job_spec.rb` | Happy path → items persisted + broadcast + ack!; unknown job_id → ack!; duplicate delivery → ack! (idempotency guard); error → reject! |
| `spec/controllers/api/v1/feeds_controller_spec.rb` | Valid request → 202 + RabbitMQ publish called; missing urls → 422; unauthenticated → 401 |
| `spec/controllers/api/v1/feed_items_controller_spec.rb` | Returns only current user's items; empty → [] |
| `spec/channels/feed_channel_spec.rb` | Subscribes to correct stream; rejects unauthenticated |

### Jest + React Testing Library — Frontend

| File | Tests |
|---|---|
| `LoginForm.test.jsx` | Renders form; submits credentials; shows error on failure |
| `FeedForm.test.jsx` | Adds/removes URL inputs; disables submit when empty; calls API on submit |
| `FeedList.test.jsx` | Renders items sorted by date; shows empty state; shows pending badge |
| `FeedItem.test.jsx` | Renders all fields; formats publish_date as YYYY-MM-DD; truncates long description |
| `useFeedChannel.test.js` | Subscribes on mount; calls onMessage with broadcast data; unsubscribes on unmount |

### Integration tests (RSpec, tag: `:integration`)

```ruby
# spec/integration/feed_submission_spec.rb
```

- Docker Compose brings up Redis + RabbitMQ + rss_service.
- `POST /api/v1/feeds` → Controller publishes → AMQP publish → Go worker → AMQP publish → `ProcessFeedResultJob` (Sneakers) → ActionCable broadcast → items persisted.
- `GET /api/v1/feed_items` returns cached items.

---

## Capybara Integration Tests

### Setup (`spec/support/capybara.rb`)

```ruby
require "capybara/rspec"
require "selenium-webdriver"

Capybara.register_driver :chrome_headless do |app|
  options = Selenium::WebDriver::Chrome::Options.new
  options.add_argument("--headless=new")
  options.add_argument("--no-sandbox")
  options.add_argument("--disable-dev-shm-usage")
  options.add_argument("--disable-gpu")
  options.add_argument("--window-size=1280,800")
  Capybara::Selenium::Driver.new(app, browser: :chrome, options: options)
end

Capybara.javascript_driver  = :chrome_headless
Capybara.default_driver     = :rack_test          # non-JS specs
Capybara.default_max_wait_time = 10               # generous wait for ActionCable
Capybara.server_host = "0.0.0.0"
Capybara.app_host    = "http://#{ENV.fetch('APP_HOST', 'localhost')}:#{Capybara.server_port}"
```

### Database Cleaner (`spec/support/database_cleaner.rb`)

```ruby
RSpec.configure do |config|
  config.before(:suite) { DatabaseCleaner.strategy = :truncation }
  config.before(:each)  { DatabaseCleaner.start }
  config.after(:each)   { DatabaseCleaner.clean }
end
```

Truncation (not transactions) is required because Selenium runs in a separate thread — transactional rollback would make fixture data invisible to the browser.

### Fixtures (`spec/fixtures/`)

**`users.yml`**
```yaml
alice:
  email: alice@example.com
  encrypted_password: <%= BCrypt::Password.create("password") %>

bob:
  email: bob@example.com
  encrypted_password: <%= BCrypt::Password.create("password") %>
```

**`feed_requests.yml`**
```yaml
alice_pending:
  user: alice
  job_id: "01J3KPENDING000"
  urls: "{https://feeds.bbci.co.uk/news/rss.xml}"
  status: pending

alice_done:
  user: alice
  job_id: "01J3KDONE000000"
  urls: "{https://feeds.bbci.co.uk/news/rss.xml}"
  status: done
```

**`feed_items.yml`**
```yaml
item_one:
  feed_request: alice_done
  title: "Breaking News"
  source: "BBC News"
  source_url: "https://feeds.bbci.co.uk/news/rss.xml"
  link: "https://bbc.co.uk/news/1"
  publish_date: "2026-05-23"
  description: "Something happened today."

item_two:
  feed_request: alice_done
  title: "Other Story"
  source: "BBC News"
  source_url: "https://feeds.bbci.co.uk/news/rss.xml"
  link: "https://bbc.co.uk/news/2"
  publish_date: "2026-05-22"
  description: "Another thing happened."
```

---

### Feature Specs (browser — Selenium)

#### `spec/features/authentication_spec.rb`

```ruby
RSpec.describe "Authentication", type: :feature, js: true do
  fixtures :users

  scenario "user can log in with valid credentials" do
    visit "/"
    expect(page).to have_current_path("/login")

    fill_in "Email",    with: users(:alice).email
    fill_in "Password", with: "password"
    click_button "Sign in"

    expect(page).to have_current_path("/feeds")
    expect(page).to have_text("Signed in successfully")
  end

  scenario "user sees error with invalid credentials" do
    visit "/login"
    fill_in "Email",    with: "wrong@example.com"
    fill_in "Password", with: "wrong"
    click_button "Sign in"

    expect(page).to have_text("Invalid email or password")
    expect(page).to have_current_path("/login")
  end

  scenario "user can log out" do
    sign_in_as users(:alice)
    click_button "Sign out"

    expect(page).to have_current_path("/login")
  end

  scenario "unauthenticated user is redirected to login" do
    visit "/feeds"
    expect(page).to have_current_path("/login")
  end
end
```

---

#### `spec/features/feed_submission_spec.rb`

Runs against the full Docker Compose stack (real Redis + RabbitMQ + rss_service).

```ruby
RSpec.describe "Feed submission (Redis + RabbitMQ + rss_service)",
               type: :feature, js: true, integration: true do
  fixtures :users

  before { sign_in_as users(:alice) }

  scenario "user submits a URL and sees parsed items via ActionCable" do
    visit "/feeds"

    fill_in "Feed URL", with: "https://feeds.bbci.co.uk/news/rss.xml"
    click_button "Parse Feeds"

    # Optimistic UI — processing badge appears immediately
    expect(page).to have_css("[data-status='processing']")

    # ActionCable broadcast arrives — wait up to 15s for real rss_service
    using_wait_time(15) do
      expect(page).to have_css("[data-status='done']")
      expect(page).to have_css(".feed-item", minimum: 1)
    end

    # Items are persisted
    expect(FeedItem.count).to be > 0
  end

  scenario "user submits multiple URLs and all results appear" do
    visit "/feeds"

    fill_in "Feed URL 1", with: "https://feeds.bbci.co.uk/news/rss.xml"
    click_button "Add another URL"
    fill_in "Feed URL 2", with: "https://rss.nytimes.com/services/xml/rss/nyt/HomePage.xml"
    click_button "Parse Feeds"

    using_wait_time(20) do
      expect(page).to have_css("[data-status='done']")
      expect(page).to have_css(".feed-item", minimum: 2)
    end
  end

  scenario "one invalid URL causes partial failure — valid items still appear" do
    visit "/feeds"

    fill_in "Feed URL 1", with: "https://feeds.bbci.co.uk/news/rss.xml"
    click_button "Add another URL"
    fill_in "Feed URL 2", with: "https://not-a-real-feed.invalid"
    click_button "Parse Feeds"

    using_wait_time(15) do
      expect(page).to have_css("[data-status='done']")
      expect(page).to have_css(".feed-item", minimum: 1)
      expect(page).to have_css(".error-banner")    # partial error shown
    end
  end

  scenario "items are sorted newest first" do
    visit "/feeds"
    fill_in "Feed URL", with: "https://feeds.bbci.co.uk/news/rss.xml"
    click_button "Parse Feeds"

    using_wait_time(15) do
      expect(page).to have_css("[data-status='done']")
    end

    dates = all(".feed-item [data-publish-date]").map { |el| Date.parse(el.text) }
    expect(dates).to eq(dates.sort.reverse)
  end
end
```

---

#### `spec/features/feed_items_display_spec.rb`

Uses fixtures — no real network or Redis required (`:rack_test` driver).

```ruby
RSpec.describe "Feed items display", type: :feature do
  fixtures :users, :feed_requests, :feed_items

  before { sign_in_as users(:alice) }

  scenario "user sees their cached feed items" do
    visit "/feeds"

    expect(page).to have_css(".feed-item", count: 2)
    expect(page).to have_text("Breaking News")
    expect(page).to have_text("Other Story")
  end

  scenario "items are displayed with correct fields" do
    visit "/feeds"

    within(".feed-item", text: "Breaking News") do
      expect(page).to have_text("BBC News")
      expect(page).to have_text("2026-05-23")
      expect(page).to have_text("Something happened today")
      expect(page).to have_link("Read more", href: "https://bbc.co.uk/news/1")
    end
  end

  scenario "items from other users are not visible" do
    sign_in_as users(:bob)
    visit "/feeds"

    expect(page).to have_css(".feed-item", count: 0)
    expect(page).to have_text("No feeds yet")
  end

  scenario "items are sorted newest first" do
    visit "/feeds"

    titles = all(".feed-item h2").map(&:text)
    expect(titles).to eq(["Breaking News", "Other Story"])
  end
end
```

---

### Request Specs (API layer — `rack_test`)

#### `spec/requests/feeds_request_spec.rb`

```ruby
RSpec.describe "POST /api/v1/feeds", type: :request do
  fixtures :users

  let(:headers) { { "Content-Type" => "application/json" } }

  context "when authenticated" do
    before { sign_in users(:alice) }

    it "returns 202 with job_id" do
      post "/api/v1/feeds",
        params: { urls: ["https://feeds.bbci.co.uk/news/rss.xml"] }.to_json,
        headers: headers

      expect(response).to have_http_status(:accepted)
      expect(json["status"]).to eq("processing")
      expect(json["job_id"]).to be_present
    end

    it "creates a FeedRequest record" do
      expect {
        post "/api/v1/feeds",
          params: { urls: ["https://example.com/rss"] }.to_json,
          headers: headers
      }.to change(FeedRequest, :count).by(1)

      expect(FeedRequest.last.status).to eq("processing")
    end

    it "returns 422 when urls is empty" do
      post "/api/v1/feeds",
        params: { urls: [] }.to_json,
        headers: headers

      expect(response).to have_http_status(:unprocessable_entity)
    end

    it "returns 422 when urls is missing" do
      post "/api/v1/feeds",
        params: {}.to_json,
        headers: headers

      expect(response).to have_http_status(:unprocessable_entity)
    end
  end

  context "when unauthenticated" do
    it "returns 401" do
      post "/api/v1/feeds",
        params: { urls: ["https://example.com/rss"] }.to_json,
        headers: headers

      expect(response).to have_http_status(:unauthorized)
    end
  end
end
```

---

#### `spec/requests/feed_items_request_spec.rb`

```ruby
RSpec.describe "GET /api/v1/feed_items", type: :request do
  fixtures :users, :feed_requests, :feed_items

  context "when authenticated as alice" do
    before { sign_in users(:alice) }

    it "returns alice's feed items" do
      get "/api/v1/feed_items"

      expect(response).to have_http_status(:ok)
      expect(json["items"].length).to eq(2)
    end

    it "returns items sorted by publish_date descending" do
      get "/api/v1/feed_items"

      dates = json["items"].map { |i| Date.parse(i["publish_date"]) }
      expect(dates).to eq(dates.sort.reverse)
    end

    it "returns correct item fields" do
      get "/api/v1/feed_items"

      item = json["items"].first
      expect(item.keys).to match_array(
        %w[title source source_url link publish_date description]
      )
    end
  end

  context "when authenticated as bob (no items)" do
    before { sign_in users(:bob) }

    it "returns an empty items array" do
      get "/api/v1/feed_items"

      expect(response).to have_http_status(:ok)
      expect(json["items"]).to eq([])
    end
  end

  context "when unauthenticated" do
    it "returns 401" do
      get "/api/v1/feed_items"
      expect(response).to have_http_status(:unauthorized)
    end
  end
end
```

---

### Capybara Gems to Add

| Gem | Purpose |
|---|---|
| `capybara` | Integration test framework |
| `selenium-webdriver` | Chrome headless driver |
| `database_cleaner-active_record` | Truncation between specs |

---

### Running Capybara Tests

```bash
# Feature + request specs only
bundle exec rspec spec/features spec/requests

# Full stack integration (requires Docker Compose up)
docker compose up -d
bundle exec rspec spec/features --tag integration

# All specs
bundle exec rspec
```

---

### Gemfile

| Gem | Purpose |
|---|---|
| `slim-rails` | Slim view engine for layouts and Devise views |
| `sneakers` | RabbitMQ AMQP consumer worker (Sidekiq-style) |
| `bunny` | AMQP 0-9-1 client for RabbitMQ publishing |
| `capybara` | Feature + request integration tests |
| `selenium-webdriver` | Chrome headless driver for Capybara |
| `database_cleaner-active_record` | Truncation strategy between specs |
| `devise` | User authentication |
| `redis` | Redis client (ActionCable) |
| `pg` | PostgreSQL adapter |
| `rspec-rails` | Test framework |
| `factory_bot_rails` | Test factories |
| `webmock` | HTTP stubbing in tests |
| `shoulda-matchers` | Model validation matchers |
| `vite_rails` | Vite integration |

### package.json

| Package | Purpose |
|---|---|
| `bootstrap` | CSS + JS components (imported via Vite) |
| `react` / `react-dom` | UI framework |
| `@rails/actioncable` | ActionCable WebSocket client |
| `react-router-dom` | SPA routing |
| `jest` / `@jest/globals` | Test runner |
| `@testing-library/react` | Component testing |
| `@testing-library/jest-dom` | DOM matchers |
| `@testing-library/user-event` | User interaction simulation |
| `vite` / `@vitejs/plugin-react` | Build tooling |

---

## Docker Compose

```yaml
services:
  db:
    image: postgres:16-alpine
    environment:
      POSTGRES_PASSWORD: password
    volumes:
      - pg_data:/var/lib/postgresql/data

  rabbitmq:
    image: rabbitmq:3.13-management-alpine
    ports:
      - "5672:5672"   # AMQP
      - "15672:15672" # management UI
    environment:
      RABBITMQ_DEFAULT_USER: guest
      RABBITMQ_DEFAULT_PASS: guest

  redis:
    image: redis:7-alpine
    ports: ["6379:6379"]

  rss_service:
    build:
      context: ../rssservice
      dockerfile: Dockerfile
    environment:
      REDIS_DSN:           redis://redis:6379/0
      RABBITMQ_URL:        amqp://guest:guest@rabbitmq:5672/
      JWT_PUBLIC_KEY_PATH: /keys/ec_public.pem
    ports: ["8080:8080"]
    depends_on: [redis, rabbitmq]

  rails:
    build: .
    command: bundle exec rails server -b 0.0.0.0
    environment:
      DATABASE_URL:    postgres://postgres:password@db/rss_frontend
      REDIS_URL:       redis://redis:6379/0
      RABBITMQ_URL:    amqp://guest:guest@rabbitmq:5672/
    ports: ["3000:3000"]
    depends_on: [db, redis, rabbitmq, rss_service]

  sneakers:
    build: .
    command: bundle exec rake sneakers:run
    environment:
      DATABASE_URL: postgres://postgres:password@db/rss_frontend
      REDIS_URL:    redis://redis:6379/0
      RABBITMQ_URL: amqp://guest:guest@rabbitmq:5672/
    depends_on: [db, redis, rabbitmq]

volumes:
  pg_data:
```

---

## Implementation Order

1. `rails new rss_frontend --database=postgresql --skip-javascript` + Gemfile setup
2. Install `slim-rails` — convert `application.html.erb` → `application.html.slim`
3. `bundle exec rails generate devise:install` + `User` model + Slim Devise views
4. DB migrations — `feed_requests`, `feed_items`
5. `config/initializers/rabbitmq.rb` — Verifies RabbitMQ connection on boot, exits on failure
6. `RabbitMQPublisher` + specs (Bunny mock)
7. `ProcessFeedResultJob` (Sneakers) + specs (idempotency guard)
8. `FeedsController` + `FeedItemsController` + specs
9. `FeedChannel` + specs
10. `config/sneakers.yml`
11. Vite + React scaffold (`vite_rails` install) + `npm install bootstrap`
12. Import Bootstrap in `application.jsx`; apply Bootstrap classes to all components
13. `cable.js` + `useFeedChannel` hook + Jest test
14. `LoginForm` + `FeedForm` + `FeedList` + `FeedItem` components + Jest tests
15. `spec/support/capybara.rb` + `database_cleaner.rb`
16. `spec/fixtures/` — users, feed_requests, feed_items YAML
17. `spec/requests/` — feeds + feed_items request specs
18. `spec/features/` — authentication, feed submission flow, display specs
19. `docker-compose.yml` + `Dockerfile`
20. `README.md`

---

## README Outline

1. **Architecture overview** — diagram of Rails ↔ RabbitMQ ↔ rss_service ↔ React
2. **Prerequisites** — Ruby 3.3+, Node 20+, Docker, PostgreSQL
3. **Environment variables** — table of all required vars
4. **Running with Docker Compose** — single command startup
5. **Running locally** — DB setup, Redis, RabbitMQ, Rails server, Vite dev server
6. **Enforced RabbitMQ verification** — explanation of the boot exit condition on missing RabbitMQ
7. **Authentication** — Devise login
8. **Running Rails unit tests** — `bundle exec rspec spec/models spec/services spec/controllers spec/channels`
9. **Running React tests** — `npm test`
10. **Running Capybara feature specs** — `bundle exec rspec spec/features spec/requests`
11. **Running full stack integration tests** — `docker compose up -d && bundle exec rspec --tag integration`
