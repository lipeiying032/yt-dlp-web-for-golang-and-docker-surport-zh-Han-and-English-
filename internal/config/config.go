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

// ResolveYtDlpPath brute-force searches for libytdlp.so / yt-dlp near the executable.
// On Android, nativeLibraryDir varies wildly across vendors (arm64, arm64-v8a, etc.).
// We search baseDir, parentDir, and up to 3 levels deep — no hardcoded ABI names.
func ResolveYtDlpPath(fallback string) string {
	exePath, err := os.Executable()
	if err != nil {
		return fallback
	}
	baseDir := filepath.Dir(exePath)
	parentDir := filepath.Dir(baseDir)
	names := []string{"libytdlp.so", "yt-dlp", "yt-dlp.exe"}

	// Walk a directory up to maxDepth looking for target filenames.
	search := func(root string, maxDepth int) string {
		var found string
		filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil || found != "" {
				return filepath.SkipDir
			}
			// Enforce depth limit
			rel, _ := filepath.Rel(root, path)
			if strings.Count(rel, string(filepath.Separator)) >= maxDepth && d.IsDir() {
				return filepath.SkipDir
			}
			if d.IsDir() {
				return nil
			}
			for _, n := range names {
				if d.Name() == n {
					found = path
					return filepath.SkipDir
				}
			}
			return nil
		})
		return found
	}

	// 1) baseDir (depth 0 — direct siblings of the executable)
	if p := search(baseDir, 1); p != "" {
		log.Printf("[ResolveYtDlpPath] FOUND: %s", p)
		return p
	}
	// 2) parentDir (depth up to 3 — covers lib/*/libytdlp.so and deeper)
	if p := search(parentDir, 3); p != "" {
		log.Printf("[ResolveYtDlpPath] FOUND: %s", p)
		return p
	}

	// Not found — build diagnostic with actual directory listings
	var diag strings.Builder
	diag.WriteString(fmt.Sprintf("NOT_FOUND|exe=%s", exePath))
	for _, dir := range []string{baseDir, parentDir} {
		diag.WriteString(fmt.Sprintf("|dir=%s,files=[", dir))
		if entries, e := os.ReadDir(dir); e == nil {
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
	}
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
