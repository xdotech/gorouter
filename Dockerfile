FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o gorouter ./cmd/gorouter

FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /app/gorouter .
EXPOSE 14747
ENV PORT=14747
ENV HOSTNAME=0.0.0.0
VOLUME ["/app/data"]
CMD ["./gorouter"]
