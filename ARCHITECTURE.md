# System-Wide Integration Architecture

This document defines the architectural shift from Redis Streams to RabbitMQ to improve message durability and retry sophistication.
It establishes the protocol for the Ruby on Rails frontend and Go-based backend, ensuring high availability through dual-mode operation 
and a robust Dead Letter Exchange (DLX) strategy.

## 1. System Overview and Component Mapping

The RSS Reader is a distributed system designed for high-concurrency feed parsing. The architecture is decoupled via RabbitMQ, with Redis
serving as a fast, transient job state store and a parsed URL cache with an explicit Time-To-Live (TTL). PostgreSQL is retained exclusively
on the Rails frontend for persistent application data storage.

| Component | Language | Responsibility |
| :--- | :--- | :--- |
| **rssreader** | Go (package) | Pure library for fetching and parsing RSS 2.0/Atom feeds. |
| **rssservice** | Go (binary) | Go worker consuming from RabbitMQ; exposes REST fallback API with JWT auth. Utilizes Redis to cache parsed URLs. |
| **Rails frontend** | Ruby (Rails 8) | Orchestration, user auth, persistent result archiving in PG, and UI delivery via ActionCable hooks. |
| **RabbitMQ** | — | Primary Message Bus: Orchestrates `rss_commands` and `rss_results` via Direct exchanges. |
| **Redis** | — | Job State, Security & Caching: Manages `job:<id>` statuses, JWT blocklist, and a temporary TTL-bound cache for parsed URLs. |
| **PostgreSQL** | — | Persistence (Frontend Only): Stores structural user profiles, feed configurations, and Solid Queue data. |

### Operational Modes
* **Full Mode:** Primary asynchronous path using RabbitMQ for command and result distribution.
* **Fallback Mode:** Synchronous REST API path triggered automatically if the message bus connection drops.

---

## 2. Data Flow: RabbitMQ-Centric Topology

The system utilizes Direct Exchanges to ensure predictable routing and performance.

### Full Mode Path
1. **Rails Producer:** Generates a ULID (`job_id`) and publishes a JSON payload to the `rss_commands` (Direct) exchange using the Bunny gem.
2. **Go Consumer:** Listens on the `rss_commands_worker` queue. Upon receipt, it checks the Redis cache for fresh, pre-parsed URL results
 before initiating direct network parsing via the `rssreader` package.
3. **State & Cache Update:** The Go worker updates the `job:<id>` status in Redis to `processing` or `done` and saves valid parsing
 structures into the Redis URL cache with an explicit TTL.
4. **Go Producer:** Publishes the JSON results to the `rss_results` (Direct) exchange.
5. **Sneakers Consumer:** A Rails-side Sneakers worker consumes from `rss_results_rails`, persists the finalized items into PostgreSQL, 
and broadcasts a lightweight cache synchronization hook via ActionCable.

### Fallback Mode Path
1. **Detection:** Bypasses explicit pre-flight pings; catches inline infrastructure errors during active service operations.
2. **Direct Invocation:** Upon a runtime exception from the bus, `PublishFeedJob` bypasses the queue and calls the Go `POST /parse` endpoint directly using an ES256 JWT.
3. **Synchronous Return:** The Go service safely parses the feeds (respecting the Redis cache layer if available) and returns the payload inline.
4. **Persistence:** Rails saves the results directly to its PostgreSQL layer, bypassing the messaging layer entirely.

---

## 3. Pros, Cons, and Risks

### Pros
* **Optimized Storage Footprint:** Removing database operations from the Go service eliminates complex connection pooling overhead and storage redundancy.
* **Fast In-Memory Caching:** Using Redis with strict TTL constraints safeguards against duplicative HTTP outbound parsing cycles for identical URLs.
* **Message Durability:** RabbitMQ’s disk-persistent queues provide superior reliability over memory-bound streams.

### Cons
* **Cache Synchronization Window:** Highly volatile feeds might experience minor synchronization drift depending on the size of the configured Redis TTL window.
* **Connection Cost:** AMQP handshake latency requires active connection pooling in Rails and long-lived worker channels in Go.

### Risk Matrix

| Risk | Likelihood | Impact | Mitigation Strategy |
| :--- | :--- | :--- | :--- |
| **RabbitMQ Connection Latency** | Medium | Low | Use Bunny connection pooling in Rails; maintain long-lived channels in Go. |
| **Redis Outage (State/Cache Loss)** | Medium | High | Fallback path bypasses caching gracefully to perform direct fetches; use Redis Sentinel for resilience. |
| **DLX Congestion Loop** | Low | Medium | Implement a "Wait Queue" pattern with a maximum of 3 TTL-based retries. |

---

## 4. RabbitMQ Dead Letter Exchange (DLX) Design

To handle transient network failures or malformed feeds, the system employs the **Wait Queue Pattern**.

1. **Main Exchange:** `rss_commands` (Direct) binds to `rss_commands_worker`.
2. **Failure Handling:** If the Go worker rejects a message, it is routed to the `rss_commands_retry` exchange.
3. **Wait Queue:** The retry exchange binds to `rss_commands_wait_30s`. This queue has no consumers and is configured with:
   * `x-message-ttl`: `30000` (30 seconds)
   * `x-dead-letter-exchange`: `rss_commands`
4. **Re-Queueing:** Once the TTL expires, the message is moved back to the `rss_commands` exchange for another attempt.
5. **Final Failure:** After 3 attempts (tracked via the `x-death` header), the message is routed to `rss_commands_failed`.
6. **Daily Cleanup:** The `rss_commands_failed` queue is swept and audited daily to review malformed feed patterns and keep storage clear.
