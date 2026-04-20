FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy only the module metadata, vendored dependencies, and source tree.
# This keeps the build offline-friendly and avoids copying local caches/binaries.
COPY go.mod go.sum ./
COPY vendor ./vendor
COPY cmd ./cmd
COPY internal ./internal

# Build the trading engine as a statically linked binary.
RUN CGO_ENABLED=0 GOOS=linux go build -mod=vendor -o /bin/antigravity antigravity-engine/cmd/antigravity

# Stage 2: Minimal Runtime Image
FROM alpine:latest
WORKDIR /app

# Add ca-certificates necessary for Binance WebSockets WSS and SSL/TLS
RUN apk --no-cache add ca-certificates tzdata

# Copy built binary from builder
COPY --from=builder /bin/antigravity /app/antigravity

# Start trading bot execution
CMD ["/app/antigravity"]
