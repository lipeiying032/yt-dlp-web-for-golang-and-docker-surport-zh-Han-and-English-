package config

import (
	"log"
	"os"
	"path/filepath"
	"strconv"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	Port          string
	DownloadDir   string
	ConfigDir     string
	StaticDir     string
	MaxConcurrent int
	YtDlpPath     string
	UsePython     bool
	DefaultArgs   []string
}

// Load reads environment variables and returns a populated Config.
func Load() *Config {
	cfg := &Config{
		Port:          envOr("PORT", "8080"),
		DownloadDir:   envOr("DOWNLOAD_DIR", "./downloads"),
		ConfigDir:     envOr("CONFIG_DIR", "./config"),
		StaticDir:     envOr("STATIC_DIR", "./static"),
		MaxConcurrent: envOrInt("MAX_CONCURRENT", 2),
		YtDlpPath:     envOr("YTDLP_PATH", "yt-dlp"),
		UsePython:     envOr("YTDLP_USE_PYTHON", "false") == "true",
	}

	if cfg.YtDlpPath == "yt-dlp" && !cfg.UsePython {
		cfg.YtDlpPath = ResolveYtDlpPath(cfg.YtDlpPath)
	}

	log.Printf("[Config] YtDlpPath=%s, UsePython=%v", cfg.YtDlpPath, cfg.UsePython)

	if err := os.MkdirAll(cfg.DownloadDir, 0o755); err != nil {
		log.Fatalf("failed to create download dir %s: %v", cfg.DownloadDir, err)
	}
	if err := os.MkdirAll(cfg.ConfigDir, 0o755); err != nil {
		log.Fatalf("failed to create config dir %s: %v", cfg.ConfigDir, err)
	}

	// Default args applied to every download.
	// --newline is critical for progress parsing.
	// 403-bypass defaults that work WITHOUT curl_cffi.
	cfg.DefaultArgs = []string{
		"--newline",
		"--no-colors",
		"--ignore-errors",
		"--no-overwrites",
		"--continue",
		"--extractor-args", "youtube:player_client=android,web",
		"--sleep-interval", "2",
		"--max-sleep-interval", "6",
		"--cache-dir", filepath.Join(cfg.ConfigDir, "cache"),
		"-o", filepath.Join(cfg.DownloadDir, "%(title)s [%(id)s].%(ext)s"),
	}

	return cfg
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// ResolveYtDlpPath is a fallback for desktop/Docker. On Android, YTDLP_PATH is set by Java.
func ResolveYtDlpPath(fallback string) string {
	exePath, err := os.Executable()
	if err != nil {
		return fallback
	}
	baseDir := filepath.Dir(exePath)
	for _, name := range []string{"yt-dlp", "yt-dlp.exe"} {
		p := filepath.Join(baseDir, name)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return fallback
}

func envOrInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return fallback
}
