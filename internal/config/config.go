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

// ResolveYtDlpPath tries to find yt-dlp in the same directory as the executable.
// On Android, the executable is in nativeLibraryDir (e.g., lib/arm64-v8a/ or lib/arm64/).
// If not found, returns a diagnostic path containing the actual directory structure.
func ResolveYtDlpPath(fallback string) string {
	exePath, err := os.Executable()
	if err != nil {
		return fallback
	}
	baseDir := filepath.Dir(exePath)
	parentDir := filepath.Dir(baseDir)

	// Scan current directory
	var baseFiles []string
	if entries, err := os.ReadDir(baseDir); err == nil {
		for _, e := range entries {
			baseFiles = append(baseFiles, e.Name())
		}
	}

	// Scan parent directory
	var parentFiles []string
	if entries, err := os.ReadDir(parentDir); err == nil {
		for _, e := range entries {
			parentFiles = append(parentFiles, e.Name())
		}
	}

	// Scan grandparent directory
	grandParentDir := filepath.Dir(parentDir)
	var grandParentFiles []string
	if entries, err := os.ReadDir(grandParentDir); err == nil {
		for _, e := range entries {
			grandParentFiles = append(grandParentFiles, e.Name())
		}
	}

	log.Printf("[ResolveYtDlpPath] exePath=%s", exePath)
	log.Printf("[ResolveYtDlpPath] baseDir=%s, files=%v", baseDir, baseFiles)
	log.Printf("[ResolveYtDlpPath] parentDir=%s, files=%v", parentDir, parentFiles)
	log.Printf("[ResolveYtDlpPath] grandParentDir=%s, files=%v", grandParentDir, grandParentFiles)

	// Try direct match in baseDir
	for _, name := range []string{"libytdlp.so", "yt-dlp", "yt-dlp.exe"} {
		p := filepath.Join(baseDir, name)
		if _, err := os.Stat(p); err == nil {
			log.Printf("[ResolveYtDlpPath] FOUND in baseDir: %s", p)
			return p
		}
	}

	// Fuzzy search in parent directories
	matches, _ := filepath.Glob(filepath.Join(parentDir, "*", "libytdlp.so"))
	if len(matches) > 0 {
		log.Printf("[ResolveYtDlpPath] FOUND by Glob: %s", matches[0])
		return matches[0]
	}

	matches, _ = filepath.Glob(filepath.Join(parentDir, "*", "*", "libytdlp.so"))
	if len(matches) > 0 {
		log.Printf("[ResolveYtDlpPath] FOUND by deep Glob: %s", matches[0])
		return matches[0]
	}

	// Not found - return diagnostic info as a special path
	// This will cause an error when trying to execute, but the error message will show the directory structure
	diag := fmt.Sprintf("NOT_FOUND|exe=%s|base=%s|baseFiles=[%s]|parent=%s|parentFiles=[%s]|grandParent=%s|grandParentFiles=[%s]",
		exePath,
		baseDir, strings.Join(baseFiles, ","),
		parentDir, strings.Join(parentFiles, ","),
		grandParentDir, strings.Join(grandParentFiles, ","))
	log.Printf("[ResolveYtDlpPath] NOT FOUND, returning diagnostic: %s", diag)
	return diag
}

// debugPrintDir is no longer needed - diagnostic info is returned directly
func debugPrintDir(dir string, depth int) {
	// Deprecated: diagnostic info is now returned directly in ResolveYtDlpPath
}

func envOrInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return fallback
}
