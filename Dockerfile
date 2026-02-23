# =============================================================================
# Stage 1: Build the Go binary
# =============================================================================
FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux \
    go build -ldflags="-s -w" -trimpath -o /app/yt-dlp-web .

# =============================================================================
# Stage 2: Download static ffmpeg
# =============================================================================
FROM alpine:3.20 AS ffmpeg

RUN apk add --no-cache curl xz && \
    ARCH=$(uname -m) && \
    case "$ARCH" in \
        x86_64)  BTBN_ARCH="linux64" ; JV_ARCH="amd64" ;; \
        aarch64) BTBN_ARCH="linuxarm64" ; JV_ARCH="arm64" ;; \
        *)       echo "Unsupported arch: $ARCH" && exit 1 ;; \
    esac && \
    ( curl -sL "https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-master-latest-${BTBN_ARCH}-gpl.tar.xz" \
      | tar xJ --strip-components=2 -C /usr/local/bin/ --wildcards '*/bin/ffmpeg' '*/bin/ffprobe' ) || \
    ( echo "Primary source failed, trying fallback..." && \
      curl -sL "https://johnvansickle.com/ffmpeg/releases/ffmpeg-release-${JV_ARCH}-static.tar.xz" \
      | tar xJ --strip-components=1 -C /usr/local/bin/ --wildcards '*/ffmpeg' '*/ffprobe' )

# =============================================================================
# Stage 3: Final runtime image
# =============================================================================
FROM python:3.12-alpine

LABEL maintainer="yt-dlp-web" \
    description="Self-hosted yt-dlp web UI"

# Install yt-dlp, create non-root user, directories
RUN apk add --no-cache su-exec && \
    pip install --no-cache-dir yt-dlp && \
    addgroup -g 1000 app && \
    adduser -u 1000 -G app -h /home/app -s /bin/sh -D app && \
    mkdir -p /app/downloads /app/config && \
    chown -R app:app /app /home/app

# Copy binaries and entrypoint
COPY --from=builder /app/yt-dlp-web /app/yt-dlp-web
COPY --from=ffmpeg /usr/local/bin/ffmpeg /usr/local/bin/ffmpeg
COPY --from=ffmpeg /usr/local/bin/ffprobe /usr/local/bin/ffprobe
COPY entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/entrypoint.sh

WORKDIR /app

ENV PORT=8080 \
    DOWNLOAD_DIR=/app/downloads \
    CONFIG_DIR=/app/config \
    MAX_CONCURRENT=2 \
    YTDLP_PATH=yt-dlp \
    HOME=/home/app \
    XDG_CACHE_HOME=/app/config/cache \
    XDG_CONFIG_HOME=/app/config

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -q --spider http://localhost:8080/health || exit 1

ENTRYPOINT ["/app/entrypoint.sh"]
