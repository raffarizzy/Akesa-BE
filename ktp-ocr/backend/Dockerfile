# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git ca-certificates

# Set global mirror and fallback to prevent TLS handshake timeouts on VPS
ENV GOPROXY=https://goproxy.io,https://proxy.golang.org,direct

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download || (sleep 3 && go mod download)

COPY . .

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o bin/ocr-backend main.go

# Final runtime stage
FROM alpine:3.20

WORKDIR /app

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /app/bin/ocr-backend /app/ocr-backend

EXPOSE 8081

CMD ["/app/ocr-backend"]
