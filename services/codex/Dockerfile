FROM golang:1.22-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o codex ./cmd/codex

FROM alpine:3.19
RUN apk --no-cache add ca-certificates
COPY --from=builder /app/codex /usr/local/bin/codex

EXPOSE 8700 8701 8702

ENTRYPOINT ["codex"]
