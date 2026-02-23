package config

import (
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
	UsePython     bool
	PythonPath    string
	DefaultArgs   []string
}

// Load reads environment variables and returns a populated Config.
func Load() *Config {
	termuxPython := "/data/data/com.termux/files/usr/bin/python3"

	cfg := &Config{
		Port:          envOr("PORT", "8080"),
		DownloadDir:   envOr("DOWNLOAD_DIR", "./downloads"),
		ConfigDir:     envOr("CONFIG_DIR", "./config"),
		StaticDir:     envOr("STATIC_DIR", "./static"),
		MaxConcurrent: envOrInt("MAX_CONCURRENT", 2),
		YtDlpPath:     envOr("YTDLP_PATH", "yt-dlp"),
		PythonPath:    envOr("PYTHON_PATH", termuxPython),
	}

	// Check USE_PYTHON env var or detect from file extension
	usePythonEnv := envOr("USE_PYTHON", "false")
	if usePythonEnv == "true" || strings.HasSuffix(cfg.YtDlpPath, ".py") {
		cfg.UsePython = true
	}

	// If YTDLP_PATH is just "yt-dlp" (not set), try to resolve it
	if cfg.YtDlpPath == "yt-dlp" {
		cfg.YtDlpPath = ResolveYtDlpPath(cfg.YtDlpPath)
	}

	log.Printf("[Config] YtDlpPath=%s, UsePython=%v, PythonPath=%s", cfg.YtDlpPath, cfg.UsePython, cfg.PythonPath)

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

// ResolveYtDlpPath finds yt-dlp binary. On Android, first checks for Termux Python.
func ResolveYtDlpPath(fallback string) string {
	// Check Termux first (if user has Termux installed with python3)
	termuxPython := "/data/data/com.termux/files/usr/bin/python3"
	if _, err := os.Stat(termuxPython); err == nil {
		// Check for yt-dlp in Termux
		termuxYtDlp := "/data/data/com.termux/files/usr/bin/yt-dlp"
		if _, err := os.Stat(termuxYtDlp); err == nil {
			log.Printf("[ResolveYtDlpPath] FOUND Termux yt-dlp: %s", termuxYtDlp)
			return termuxYtDlp
		}
		// Fallback: use Termux python + yt-dlp.py from our assets
		log.Printf("[ResolveYtDlpPath] Found Termux Python: %s", termuxPython)
	}

	// Desktop/Docker fallback
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
