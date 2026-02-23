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
// On Android, the executable is in nativeLibraryDir (e.g., lib/arm64-v8a/ or lib/arm64/).
// This function handles multiple possible extraction paths for different Android ROMs.
func ResolveYtDlpPath(fallback string) string {
	// 1. Get current .so running directory
	exePath, err := os.Executable()
	if err != nil {
		log.Printf("[ResolveYtDlpPath] os.Executable() error: %v", err)
		return fallback
	}
	baseDir := filepath.Dir(exePath)
	log.Printf("[ResolveYtDlpPath] exePath=%s, baseDir=%s", exePath, baseDir)

	// Debug: print all files in baseDir and parent directories
	debugPrintDir(baseDir, 0)
	parentDir := filepath.Dir(baseDir)
	log.Printf("[ResolveYtDlpPath] parentDir=%s", parentDir)
	debugPrintDir(parentDir, 1)
	grandParentDir := filepath.Dir(parentDir)
	log.Printf("[ResolveYtDlpPath] grandParentDir=%s", grandParentDir)
	debugPrintDir(grandParentDir, 2)

	// 2. Construct high-priority search list, covering arm64-v8a physical path
	searchPaths := []string{
		filepath.Join(baseDir, "libytdlp.so"),                    // Scenario A: directly in current directory
		filepath.Join(baseDir, "..", "arm64-v8a", "libytdlp.so"), // Scenario B: in sibling v8a directory
		filepath.Join(baseDir, "..", "arm64", "libytdlp.so"),     // Scenario C: in sibling shorthand directory
	}

	for _, p := range searchPaths {
		absP, _ := filepath.Abs(p)
		log.Printf("[ResolveYtDlpPath] trying path: %s (abs: %s)", p, absP)
		if _, err := os.Stat(p); err == nil {
			log.Printf("[ResolveYtDlpPath] FOUND: %s", p)
			return p
		} else {
			log.Printf("[ResolveYtDlpPath] stat error: %v", err)
		}
	}

	// 3. Ultimate fuzzy search: recursively find all libytdlp.so files under nativeLibraryDir's parent directory
	// This solves the problem of inconsistent extraction paths across different manufacturers
	matches, _ := filepath.Glob(filepath.Join(parentDir, "*", "libytdlp.so"))
	log.Printf("[ResolveYtDlpPath] Glob matches in parentDir/*: %v", matches)
	if len(matches) > 0 {
		return matches[0]
	}

	// Also try deeper search
	matches, _ = filepath.Glob(filepath.Join(parentDir, "*", "*", "libytdlp.so"))
	log.Printf("[ResolveYtDlpPath] Glob matches in parentDir/*/*: %v", matches)
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

	log.Printf("[ResolveYtDlpPath] NOT FOUND, returning fallback: %s", fallback)
	return fallback
}

// debugPrintDir prints all files in a directory recursively for debugging
func debugPrintDir(dir string, depth int) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		log.Printf("[debugPrintDir] depth=%d, dir=%s, error: %v", depth, dir, err)
		return
	}
	log.Printf("[debugPrintDir] depth=%d, dir=%s, entries=%d", depth, dir, len(entries))
	for _, entry := range entries {
		fullPath := filepath.Join(dir, entry.Name())
		if entry.IsDir() {
			log.Printf("[debugPrintDir]   [DIR]  %s", entry.Name())
			if depth < 2 {
				debugPrintDir(fullPath, depth+1)
			}
		} else {
			info, _ := entry.Info()
			log.Printf("[debugPrintDir]   [FILE] %s (size=%d)", entry.Name(), info.Size())
		}
	}
}

func envOrInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return fallback
}
