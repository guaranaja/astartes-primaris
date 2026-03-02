FROM golang:1.22-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o logis ./cmd/logis

FROM alpine:3.19
RUN apk --no-cache add ca-certificates
COPY --from=builder /app/logis /usr/local/bin/logis

EXPOSE 8600 8601 8602

ENTRYPOINT ["logis"]
