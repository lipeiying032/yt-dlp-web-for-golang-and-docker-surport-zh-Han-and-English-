# =============================================================================
# Stage 1: Build the Go binary
# =============================================================================
FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git

WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download 2>/dev/null || true
COPY . .
RUN go mod tidy && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -trimpath -o /app/yt-dlp-web .

# =============================================================================
# Stage 2: Download static ffmpeg
# =============================================================================
FROM alpine:3.20 AS ffmpeg

RUN apk add --no-cache curl xz && \
    ARCH=$(uname -m) && \
    if [ "$ARCH" = "x86_64" ]; then FFARCH="amd64"; else FFARCH="arm64"; fi && \
    curl -sL "https://johnvansickle.com/ffmpeg/releases/ffmpeg-release-${FFARCH}-static.tar.xz" \
    | tar xJ --strip-components=1 -C /usr/local/bin/

# =============================================================================
# Stage 3: Final runtime image
# =============================================================================
FROM python:3.12-alpine

LABEL maintainer="yt-dlp-web" \
    description="Self-hosted yt-dlp web UI"

# Install yt-dlp, create non-root user, directories
RUN pip install --no-cache-dir -U yt-dlp && \
    addgroup -g 1000 app && \
    adduser -u 1000 -G app -h /home/app -s /bin/sh -D app && \
    mkdir -p /app/downloads /app/config /app/static && \
    chown -R app:app /app /home/app

# Copy binaries
COPY --from=builder /app/yt-dlp-web /app/yt-dlp-web
COPY --from=ffmpeg /usr/local/bin/ffmpeg /usr/local/bin/ffmpeg
COPY --from=ffmpeg /usr/local/bin/ffprobe /usr/local/bin/ffprobe
COPY --chown=app:app static/ /app/static/

WORKDIR /app

ENV PORT=8080 \
    DOWNLOAD_DIR=/app/downloads \
    CONFIG_DIR=/app/config \
    STATIC_DIR=/app/static \
    MAX_CONCURRENT=2 \
    YTDLP_PATH=yt-dlp \
    HOME=/home/app \
    XDG_CACHE_HOME=/app/config/cache \
    XDG_CONFIG_HOME=/app/config

USER app
EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --retries=3 \
    CMD wget -q --spider http://localhost:8080/health || exit 1

# Default: start web server. Pass args to use as CLI wrapper:
#   docker run yt-dlp-web https://youtube.com/watch?v=xxx
ENTRYPOINT ["/app/yt-dlp-web"]
