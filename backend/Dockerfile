FROM golang:1.25 AS builder
WORKDIR /app/backend

# Toolchain + sqlite dev headers für CGO + FTS5
RUN apt-get update && apt-get install -y --no-install-recommends \
    gcc g++ libc6-dev pkg-config libsqlite3-dev \
  && rm -rf /var/lib/apt/lists/*

COPY . /app/backend

RUN ls -la /app/backend && ls -la /app/backend/sqlite && head -n 5 /app/backend/go.mod
RUN go env GOMOD && pwd

RUN go mod download

# WICHTIG: sqlite_fts5 Tag aktivieren
RUN CGO_ENABLED=1 GOOS=linux go build -tags "sqlite_fts5" -trimpath -ldflags="-s -w" \
  -o /out/audio-transcript-server .

FROM debian:stable-slim
WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates curl jq \
  && rm -rf /var/lib/apt/lists/*

COPY --from=builder /out/audio-transcript-server /usr/local/bin/audio-transcript-server
RUN chmod +x /usr/local/bin/audio-transcript-server

ENTRYPOINT ["/usr/local/bin/audio-transcript-server"]