FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy all source files
COPY . .

# Download dependencies
RUN go mod tidy

# Build the trading engine as a statically linked binary
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/antigravity ./cmd/antigravity

# Stage 2: Minimal Runtime Image
FROM alpine:latest
WORKDIR /app

# Add ca-certificates necessary for Binance WebSockets WSS and SSL/TLS
RUN apk --no-cache add ca-certificates tzdata

# Copy built binary from builder
COPY --from=builder /bin/antigravity /app/antigravity

# Start trading bot execution
CMD ["/app/antigravity"]
