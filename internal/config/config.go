package config

import (
	"os"
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
	}

	// Try to find yt-dlp in the same directory as the executable, useful for portable zero-config setups
	if cfg.YtDlpPath == "yt-dlp" {
		if exePath, err := os.Executable(); err == nil {
			importPath := func(name string) string {
				// Get the directory of the executable
				for i := len(exePath) - 1; i >= 0 && !os.IsPathSeparator(exePath[i]); i-- {
					if i == 0 {
						return name
					}
				}
				// extract dir safely
				dir := ""
				for i := len(exePath) - 1; i >= 0; i-- {
					if os.IsPathSeparator(exePath[i]) {
						dir = exePath[:i]
						break
					}
				}
				return dir + string(os.PathSeparator) + name
			}
			if _, err := os.Stat(importPath("yt-dlp.exe")); err == nil {
				cfg.YtDlpPath = importPath("yt-dlp.exe")
			} else if _, err := os.Stat(importPath("yt-dlp")); err == nil {
				cfg.YtDlpPath = importPath("yt-dlp")
			}
		}
	}

	_ = os.MkdirAll(cfg.DownloadDir, 0o755)
	_ = os.MkdirAll(cfg.ConfigDir, 0o755)

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
		"--cache-dir", cfg.ConfigDir + "/cache",
		"-o", cfg.DownloadDir + "/%(title)s [%(id)s].%(ext)s",
	}

	return cfg
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
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
