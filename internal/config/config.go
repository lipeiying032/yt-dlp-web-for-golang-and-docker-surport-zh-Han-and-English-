package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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

	if cfg.YtDlpPath == "yt-dlp" {
		cfg.YtDlpPath = ResolveYtDlpPath(cfg.YtDlpPath)
	}

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

// ResolveYtDlpPath finds yt-dlp binary. On Android the Java layer sets YTDLP_PATH
// after extracting from assets, so this is mainly a fallback for desktop/Docker.
func ResolveYtDlpPath(fallback string) string {
	exePath, err := os.Executable()
	if err != nil {
		return fallback
	}
	baseDir := filepath.Dir(exePath)

	// Check common locations near the executable
	for _, name := range []string{"yt-dlp", "yt-dlp.exe", "libytdlp.so"} {
		p := filepath.Join(baseDir, name)
		if _, err := os.Stat(p); err == nil {
			log.Printf("[ResolveYtDlpPath] FOUND: %s", p)
			return p
		}
	}

	// Not found — return diagnostic
	var diag strings.Builder
	diag.WriteString(fmt.Sprintf("NOT_FOUND|exe=%s|dir=%s,files=[", exePath, baseDir))
	if entries, e := os.ReadDir(baseDir); e == nil {
		for i, ent := range entries {
			if i > 0 {
				diag.WriteByte(',')
			}
			name := ent.Name()
			if ent.IsDir() {
				name += "/"
			}
			diag.WriteString(name)
		}
	}
	diag.WriteByte(']')
	log.Printf("[ResolveYtDlpPath] %s", diag.String())
	return diag.String()
}

func envOrInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return fallback
}
