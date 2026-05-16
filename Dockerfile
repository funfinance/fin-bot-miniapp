FROM golang:1.22 AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build -o finbot cmd/bot/main.go

FROM debian:bookworm-slim

RUN apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates tzdata procps && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=builder /app/finbot .
COPY --from=builder /app/go.mod .
RUN mkdir -p /app/data /app/logs

HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD pgrep -f finbot || exit 1

ENTRYPOINT ["/app/finbot"]
CMD ["-config", "/app/configs/config.yaml"]
