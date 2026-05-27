# Multi-stage build for RSS Reader Service using Podman
#
# Build command (run from the workspace root):
#   podman build -t rssservice -f rssservice/Dockerfile .

# Stage 1 — builder
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Copy the entire workspace to satisfy local package replacement
COPY rss_reader/ ./rss_reader/
COPY rssservice/ ./rssservice/

# Build the service binary statically
WORKDIR /app/rssservice
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /rssservice ./cmd/rssservice

# Stage 2 — runtime
FROM alpine:3.20

# Install ca-certificates (required to fetch RSS/Atom feeds over HTTPS)
RUN apk --no-cache add ca-certificates

COPY --from=builder /rssservice /rssservice
COPY rssservice/config.yaml /config.yaml
COPY rssservice/keys/ /keys/

EXPOSE 8080

ENTRYPOINT ["/rssservice"]
