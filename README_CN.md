# yt-dlp web

[English](README.md)

一个轻量级、自托管的 [yt-dlp](https://github.com/yt-dlp/yt-dlp) Web 界面 — 基于 Go (Fiber) + Alpine.js + DaisyUI 构建。

![screenshot](https://img.shields.io/badge/状态-稳定-brightgreen) ![license](https://img.shields.io/badge/许可证-MIT-blue)

## ✨ 功能特色

- **完整 yt-dlp 功能** — 15 个选项组全部可视化操作，另有原始命令模式供高级用户使用
- **实时进度** — WebSocket 驱动的实时更新，含进度条、速度、ETA 及可展开日志
- **下载队列** — 并发工作池，支持暂停 / 继续 / 重试 / 取消 / 删除
- **格式列表** — 一键查看任意 URL 的 `yt-dlp -F` 输出
- **认证支持** — YouTube OAuth2、从浏览器导入 Cookies、用户名/密码
- **后处理** — 音频提取、转封装、转码、嵌入字幕/封面/元数据/章节、SponsorBlock
- **双语界面** — 中文 / English 一键切换，自动检测浏览器语言
- **深色/浅色主题** — 自定义薄荷天空配色 DaisyUI 主题，毛玻璃卡片效果
- **Docker 优先** — 多阶段构建，镜像 < 200 MB，健康检查，非 root 用户
- **CLI 回退** — 直接传参：`docker run yt-dlp-web https://...`

## 🚀 快速开始

```bash
git clone https://github.com/<your-user>/yt-dlp-web.git
cd yt-dlp-web
docker compose up -d
# 打开 http://localhost:8080
```

## ⚙️ 环境变量

| 变量 | 默认值 | 说明 |
|---|---|---|
| `PORT` | `8080` | Web 服务端口 |
| `DOWNLOAD_DIR` | `/app/downloads` | 文件保存路径 |
| `CONFIG_DIR` | `/app/config` | OAuth 令牌和缓存 |
| `MAX_CONCURRENT` | `2` | 并行下载工作线程数 |
| `YTDLP_PATH` | `yt-dlp` | yt-dlp 可执行文件路径 |

## 🔐 YouTube OAuth2 认证

1. 在认证面板中将 **用户名** 设为 `oauth2`
2. 开始下载 — 日志中会显示设备授权码
3. 在浏览器中打开提示的 URL 并输入授权码
4. 令牌会缓存在 `CONFIG_DIR` 中，后续使用无需再次授权

## 🏗️ 项目结构

```
main.go                  → Fiber 服务器、WS 升级、CLI 回退
internal/config/         → 基于环境变量的配置
internal/download/       → 任务模型、进度解析器、工作池
internal/handler/        → REST API（10 个端点）+ WebSocket 中心
internal/params/         → 30+ 字段请求 → yt-dlp 参数映射
static/index.html        → Alpine.js 单页应用（含国际化）
Dockerfile               → 三阶段构建（Go + ffmpeg + yt-dlp）
docker-compose.yml       → 一键部署
```

## 📝 开源许可

MIT
