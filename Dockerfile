# ─── Build Stage ──────────────────────────────────────────────────────────────
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o gorouter ./cmd/gorouter

# ─── Runtime Stage ────────────────────────────────────────────────────────────
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata curl \
    && addgroup -S gorouter && adduser -S gorouter -G gorouter

WORKDIR /app
COPY --from=builder /app/gorouter .

# Run as non-root
RUN chown -R gorouter:gorouter /app
USER gorouter

EXPOSE 14747

ENV PORT=14747
ENV HOSTNAME=0.0.0.0
ENV DATA_DIR=/app/data

VOLUME ["/app/data"]

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:14747/api/init || exit 1

CMD ["./gorouter"]
