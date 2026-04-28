FROM golang:1.21-alpine AS builder

WORKDIR /app

COPY go.mod ./
COPY cmd/antigravity ./cmd/antigravity
COPY internal ./internal

RUN go mod tidy && CGO_ENABLED=0 GOOS=linux go build -o /bin/antigravity antigravity-engine/cmd/antigravity

FROM alpine:latest
WORKDIR /app

RUN apk --no-cache add ca-certificates tzdata

COPY --from=builder /bin/antigravity /app/antigravity

CMD ["/app/antigravity"]
