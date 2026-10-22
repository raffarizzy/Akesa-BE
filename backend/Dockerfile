# Build stage
FROM golang:alpine AS builder

WORKDIR /app

# Install git and build dependencies
RUN apk add --no-cache git ca-certificates tzdata

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

# Compile static Go binary
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o bin/api ./cmd/api

# Final runtime stage
FROM alpine:3.20

WORKDIR /app

# Create directory for patient identity storage
RUN mkdir -p /app/storage/identity

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

COPY --from=builder /app/bin/api /app/api
COPY --from=builder /app/migrations /app/migrations

EXPOSE 8080

CMD ["/app/api"]
