FROM golang:1.22-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o primarch ./cmd/primarch

FROM alpine:3.19
RUN apk --no-cache add ca-certificates
COPY --from=builder /app/primarch /usr/local/bin/primarch

EXPOSE 8400 8401 8402

ENTRYPOINT ["primarch"]
