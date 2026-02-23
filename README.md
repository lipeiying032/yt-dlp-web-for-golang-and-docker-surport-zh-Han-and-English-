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

### Option 1: Download Pre-built Binary

Download the latest release for your platform from [GitHub Releases](https://github.com/lipeiying032/yt-dlp-web-for-golang-and-docker-surport-zh-Han-and-English-/releases):

| Platform | Architecture | File |
|---|---|---|
| Windows | x64 (amd64) | `yt-dlp-web-windows-amd64.zip` |
| Windows | x86 (32-bit) | `yt-dlp-web-windows-386.zip` |
| Windows | ARM64 | `yt-dlp-web-windows-arm64.zip` |
| macOS | Intel (amd64) | `yt-dlp-web-darwin-amd64.tar.gz` |
| macOS | Apple Silicon | `yt-dlp-web-darwin-arm64.tar.gz` |

**Prerequisites:** [yt-dlp](https://github.com/yt-dlp/yt-dlp) and [ffmpeg](https://ffmpeg.org/) must be installed and on your PATH.

```bash
# Extract and run (no installation needed)
./yt-dlp-web          # macOS/Linux
yt-dlp-web.exe        # Windows

# Open http://localhost:8080
```

### Option 2: Docker

```bash
git clone https://github.com/lipeiying032/yt-dlp-web-for-golang-and-docker-surport-zh-Han-and-English-.git
cd yt-dlp-web-for-golang-and-docker-surport-zh-Han-and-English-
docker compose up -d
# Open http://localhost:8080
```

### Build from Source

```bash
git clone https://github.com/lipeiying032/yt-dlp-web-for-golang-and-docker-surport-zh-Han-and-English-.git
cd yt-dlp-web-for-golang-and-docker-surport-zh-Han-and-English-
go build -ldflags="-s -w" -trimpath -o yt-dlp-web .
./yt-dlp-web
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

## 📱 Mobile App (Android)

This project includes a simple Android native wrapper (WebView). Since binaries are not bundled in the repo for local builds, you must prepare them before building the APK:

1. Ensure **Go** is installed and in your PATH.
2. Run the preparation script (Windows):
   ```powershell
   .\scripts\prepare-android.ps1
   ```
3. Open the `android` directory in **Android Studio**, then build and run.

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
