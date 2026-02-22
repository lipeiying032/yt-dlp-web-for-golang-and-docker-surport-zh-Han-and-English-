# yt-dlp web

[中文文档](README_CN.md)

A lightweight, self-hosted web UI for [yt-dlp](https://github.com/yt-dlp/yt-dlp) — built with Go (Fiber) + Alpine.js + DaisyUI.

![screenshot](https://img.shields.io/badge/status-stable-brightgreen) ![license](https://img.shields.io/badge/license-MIT-blue)

## ✨ Features

- **Full yt-dlp parity** — all 15 option groups exposed as visual controls, plus raw command mode for advanced users
- **Real-time progress** — WebSocket-powered live updates with progress bars, speed, ETA & expandable logs
- **Download queue** — concurrent worker pool, pause / resume / retry / cancel / delete
- **Format lister** — one-click `yt-dlp -F` output for any URL
- **Authentication** — YouTube OAuth2, cookies-from-browser, username/password
- **Post-processing** — audio extraction, remux, recode, embed subs/thumb/metadata/chapters, SponsorBlock
- **Bilingual UI** — English / 中文 toggle with auto-detection
- **Dark / light theme** — custom mint-sky DaisyUI themes with glassy cards
- **Docker-first** — multi-stage build, <200 MB image, healthcheck, non-root user
- **CLI fallback** — pass args directly: `docker run yt-dlp-web https://...`

## 🚀 Quick Start

```bash
git clone https://github.com/<your-user>/yt-dlp-web.git
cd yt-dlp-web
docker compose up -d
# Open http://localhost:8080
```

## ⚙️ Environment Variables

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | Web server port |
| `DOWNLOAD_DIR` | `/app/downloads` | Where files are saved |
| `CONFIG_DIR` | `/app/config` | OAuth tokens & cache |
| `MAX_CONCURRENT` | `2` | Parallel download workers |
| `YTDLP_PATH` | `yt-dlp` | Path to yt-dlp binary |

## 🔐 YouTube OAuth2

1. Set **Username** to `oauth2` in the Authentication panel
2. Start a download — logs will show a device code
3. Open the URL in your browser and enter the code
4. Token is cached in `CONFIG_DIR` for future use

## 🏗️ Architecture

```
main.go                  → Fiber server, WS upgrade, CLI fallback
internal/config/         → ENV-based configuration
internal/download/       → Task model, progress parser, worker pool
internal/handler/        → REST API (10 endpoints) + WebSocket hub
internal/params/         → 30+ field request → yt-dlp args mapper
static/index.html        → Alpine.js SPA with i18n
Dockerfile               → 3-stage build (Go + ffmpeg + yt-dlp)
docker-compose.yml       → One-command deployment
```

## 📝 License

MIT
