# ── Build stage ───────────────────────────────────────────────────────────────
FROM golang:1.22-bullseye AS builder

WORKDIR /app

# Copy dependency files first for layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build the main bot binary (CGO required for SQLite)
RUN CGO_ENABLED=1 GOOS=linux go build -o prophet-bot ./cmd/bot

# ── Runtime stage ─────────────────────────────────────────────────────────────
FROM debian:bullseye-slim

# SQLite runtime dependency
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    libsqlite3-0 \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copy binary
COPY --from=builder /app/prophet-bot .

# Copy web dashboard
COPY --from=builder /app/web ./web

# Default data directory (Railway volume mount or ephemeral)
RUN mkdir -p /app/data

EXPOSE 4534

CMD ["./prophet-bot"]
