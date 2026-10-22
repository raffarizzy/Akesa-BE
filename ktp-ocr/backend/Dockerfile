# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o bin/ocr-backend main.go

# Final runtime stage
FROM alpine:3.20

WORKDIR /app

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /app/bin/ocr-backend /app/ocr-backend

EXPOSE 8081

CMD ["/app/ocr-backend"]
