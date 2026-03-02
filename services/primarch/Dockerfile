# Primarch — Control plane for the Imperium
# Multi-stage build optimized for Google Cloud Run.

FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o primarch ./cmd/primarch

FROM alpine:3.20
RUN apk --no-cache add ca-certificates tzdata

# Cloud Run sets PORT env var — Primarch reads it from config.
COPY --from=builder /app/primarch /usr/local/bin/primarch

# Primarch serves: gRPC (:8400), HTTP/SSE (:8401), metrics (:8402)
# Cloud Run maps its PORT to 8401 (the HTTP API that Aurum proxies to).
EXPOSE 8401

ENTRYPOINT ["primarch"]
