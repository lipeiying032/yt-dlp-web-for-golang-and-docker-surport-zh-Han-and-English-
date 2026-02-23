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

// ResolveYtDlpPath tries to find yt-dlp in the same directory as the executable.
// On Android, the executable is in nativeLibraryDir, where jniLibs are extracted.
// This function handles multiple possible extraction paths for different Android ROMs.
func ResolveYtDlpPath(fallback string) string {
	// 1. Get current executable path (usually .../lib/arm64-v8a/ or .../lib/arm64/)
	exePath, err := os.Executable()
	if err != nil {
		return fallback
	}
	baseDir := filepath.Dir(exePath)

	// 2. Construct search priority list
	// Priority: same directory, then parent lib directory with architecture folders
	searchPaths := []string{
		filepath.Join(baseDir, "libytdlp.so"),                           // Same directory
		filepath.Join(baseDir, "..", "lib", "arm64-v8a", "libytdlp.so"), // Cross-directory fallback
		filepath.Join(baseDir, "..", "lib", "arm64", "libytdlp.so"),     // Compatible shorthand directory
	}

	for _, p := range searchPaths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	// 3. Ultimate fuzzy matching: recursively find libytdlp.so in baseDir and parent directories
	// Solves some modified ROM extraction paths
	matches, _ := filepath.Glob(filepath.Join(baseDir, "*", "libytdlp.so"))
	if len(matches) > 0 {
		return matches[0]
	}

	// Also try parent lib directories
	matches, _ = filepath.Glob(filepath.Join(baseDir, "..", "lib", "*", "libytdlp.so"))
	if len(matches) > 0 {
		return matches[0]
	}

	// 4. Cross-platform compatibility
	for _, name := range []string{"yt-dlp.exe", "yt-dlp"} {
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
