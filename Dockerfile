# syntax=docker/dockerfile:1

FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS builder
ARG TARGETOS=linux
ARG TARGETARCH=arm64

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -ldflags="-s -w" -o /out/instabox-vm-monitor ./cmd/server

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata \
 && addgroup -g 10001 -S ibmon \
 && adduser -u 10001 -S -G ibmon ibmon \
 && mkdir -p /data \
 && chown -R 10001:10001 /data

COPY --from=builder /out/instabox-vm-monitor /usr/local/bin/instabox-vm-monitor

USER 10001:10001
VOLUME ["/data"]
ENV PORT=8080 \
    DATA_DIR=/data \
    POLL_INTERVAL_SECONDS=30 \
    METRICS_RETENTION_DAYS=7

EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD ["/usr/local/bin/instabox-vm-monitor", "-healthcheck"]

ENTRYPOINT ["/usr/local/bin/instabox-vm-monitor"]
