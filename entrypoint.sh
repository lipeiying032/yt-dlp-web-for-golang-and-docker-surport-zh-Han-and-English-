#!/bin/sh
# Fix ownership of mounted volumes, then drop to app user
if [ "$(id -u)" = "0" ]; then
    chown -R app:app /app/downloads /app/config 2>/dev/null || true
    exec su-exec app /app/yt-dlp-web "$@"
fi
exec /app/yt-dlp-web "$@"
