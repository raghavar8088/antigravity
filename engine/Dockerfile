FROM golang:1.25-alpine AS builder

WORKDIR /app

# Install build dependencies.
RUN apk --no-cache add ca-certificates tzdata git

COPY go.mod ./
COPY cmd/antigravity ./cmd/antigravity
COPY internal ./internal

# Build with hardening flags: PIE, stripped symbols, no CGO.
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -mod=mod \
    -ldflags="-s -w -extldflags=-static" \
    -trimpath \
    -o /bin/antigravity ./cmd/antigravity

# ── Runtime image ─────────────────────────────────────────────────────────────
FROM scratch

# Copy trusted root certificates and timezone data from builder.
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Copy compiled binary.
COPY --from=builder /bin/antigravity /app/antigravity

# ── Security hardening ────────────────────────────────────────────────────────
# Run as non-root user UID=10001 (no /etc/passwd in scratch image — numeric UID only).
USER 10001:10001

# Expose only the application port.
EXPOSE 8080

# Memory and GC tuning.
ENV GOMEMLIMIT=450MiB
ENV GOGC=70
ENV PORT=8080

# This image is FROM scratch — no shell, no wget/curl — so a wget-based
# HEALTHCHECK (as previously used in docker-compose.prod.yml) can never
# succeed and only produces false "unhealthy" noise. The binary supports
# `--healthcheck` for an in-process self-check instead (see cmd/antigravity/main.go).
HEALTHCHECK --interval=30s --timeout=5s --retries=5 --start-period=40s \
    CMD ["/app/antigravity", "--healthcheck"]

# Secrets are injected at runtime via environment variables or Vault.
# NEVER bake API keys or passwords into this image.

# Read-only filesystem: bind-mount /data if SQLite persistence is needed.
# docker run --read-only --tmpfs /tmp --mount type=bind,src=/data,dst=/app/data

ENTRYPOINT ["/app/antigravity"]
