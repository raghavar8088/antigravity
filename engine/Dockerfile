FROM golang:1.21-alpine AS builder

WORKDIR /app

COPY go.mod ./
COPY cmd/antigravity ./cmd/antigravity
COPY internal ./internal

RUN go mod tidy && CGO_ENABLED=0 GOOS=linux go build -o /bin/antigravity ./cmd/antigravity

FROM alpine:latest
WORKDIR /app

RUN apk --no-cache add ca-certificates tzdata

COPY --from=builder /bin/antigravity /app/antigravity

ENV GOMEMLIMIT=450MiB
ENV GOGC=70
ENV PORT=8080

CMD ["/app/antigravity"]
