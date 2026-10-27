# keep in sync with combined_refactor/go.mod
ARG GO_IMAGE=golang:1.26-bookworm
ARG RUNTIME_IMAGE=alpine:3.22

FROM --platform=$BUILDPLATFORM ${GO_IMAGE} AS builder
ARG TARGETOS=linux
ARG TARGETARCH
ARG VERSION=dev

WORKDIR /src
COPY combined_refactor/go.mod combined_refactor/go.sum ./
RUN go mod download
COPY combined_refactor/ ./
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH:-$(go env GOARCH)} \
    go build -trimpath -ldflags "-s -w -X main.appVersion=${VERSION}" -o /out/cfdata .

FROM ${RUNTIME_IMAGE}
ARG VERSION=dev
ARG REVISION=unknown

LABEL org.opencontainers.image.title="CFData-WEB" \
      org.opencontainers.image.description="Cloudflare IP 测试与筛选工具（Web 模式）" \
      org.opencontainers.image.source="https://github.com/tunecc/CFData-docker" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${REVISION}" \
      org.opencontainers.image.licenses="GPL-3.0-or-later"

RUN apk add --no-cache ca-certificates tzdata su-exec \
    && mkdir -p /app /data \
    && addgroup -g 10001 -S cfdata \
    && adduser -u 10001 -S -D -H -G cfdata -h /data cfdata \
    && chown 10001:10001 /data

COPY --from=builder /out/cfdata /app/cfdata
COPY docker/docker-entrypoint.sh /app/docker-entrypoint.sh
COPY LICENSE /app/LICENSE
RUN chmod 0755 /app/cfdata /app/docker-entrypoint.sh

ENV HOME=/data \
    CFDATA_HEALTHCHECK_HOST=127.0.0.1 \
    CFDATA_HEALTHCHECK_PORT=13335

HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
    CMD wget -q -O /dev/null "http://${CFDATA_HEALTHCHECK_HOST}:${CFDATA_HEALTHCHECK_PORT}/favicon.png" || exit 1

WORKDIR /data
EXPOSE 13335

# 入口以 root 启动，仅用于把 /data 属主修正为 PUID/PGID（默认 10001:10001），
# 随后立即降权，应用进程始终以非 root 运行。
ENTRYPOINT ["/app/docker-entrypoint.sh"]
CMD []
