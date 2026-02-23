package main

import (
	"crypto/subtle"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"yt-dlp-web/internal/config"
	"yt-dlp-web/internal/download"
	"yt-dlp-web/internal/handler"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

//go:embed static/*
var staticFS embed.FS

func main() {
	// CLI fallback: if any args given, pass straight to yt-dlp
	if len(os.Args) > 1 {
		ytdlp := config.ResolveYtDlpPath("yt-dlp")
		if p := os.Getenv("YTDLP_PATH"); p != "" {
			ytdlp = p
		}
		cmd := exec.Command(ytdlp, os.Args[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		if err := cmd.Run(); err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				os.Exit(exitErr.ExitCode())
			}
			os.Exit(1)
		}
		os.Exit(0)
	}

	// --- Web server mode ---
	cfg := config.Load()

	// Sanity check for yt-dlp existence to prevent silent failing downloads
	checkYtDlp := func(path string) error {
		// If path is a diagnostic string, show it directly
		if strings.HasPrefix(path, "NOT_FOUND|") {
			return fmt.Errorf("YT-DLP NOT FOUND!\n\nDiagnostic info:\n%s", path)
		}
		if _, err := os.Stat(path); err == nil {
			return nil // Exists as absolute or relative path
		}
		_, err := exec.LookPath(path)
		return err // Fallback to system PATH check
	}

	if err := checkYtDlp(cfg.YtDlpPath); err != nil {
		log.Printf("=============================================================================")
		log.Printf("FATAL: %v", err)
		log.Printf("=============================================================================")
		// On Android, we want this error to be visible to the user
		// The Go server will still start, but downloads will fail with clear error
	}

	hub := handler.NewHub()

	mgr := download.NewManager(cfg, func(t *download.Task) {
		hub.BroadcastTask(t)
	})

	api := handler.NewAPI(mgr)

	app := fiber.New(fiber.Config{
		AppName:               "yt-dlp-web",
		DisableStartupMessage: true,
		BodyLimit:             10 * 1024 * 1024,
	})

	app.Use(recover.New())
	app.Use(logger.New(logger.Config{
		Format:     "${time} ${status} ${method} ${path} ${latency}\n",
		TimeFormat: "15:04:05",
	}))
	corsOrigins := os.Getenv("CORS_ORIGINS")
	if corsOrigins == "" {
		corsOrigins = fmt.Sprintf("http://localhost:%s, http://127.0.0.1:%s", cfg.Port, cfg.Port)
	}
	app.Use(cors.New(cors.Config{
		AllowOrigins:     corsOrigins,
		AllowMethods:     "GET,POST,DELETE",
		AllowHeaders:     "Content-Type, X-API-Key",
		AllowCredentials: false,
	}))

	// Optional API key authentication via API_KEY env var
	apiKey := os.Getenv("API_KEY")
	if apiKey != "" {
		app.Use(func(c *fiber.Ctx) error {
			// Skip auth for health check and static files
			p := c.Path()
			if p == "/health" || (!strings.HasPrefix(p, "/api/") && !strings.HasPrefix(p, "/ws")) {
				return c.Next()
			}
			if subtle.ConstantTimeCompare([]byte(c.Get("X-API-Key")), []byte(apiKey)) == 1 {
				return c.Next()
			}
			return c.Status(401).JSON(fiber.Map{"error": "unauthorized"})
		})
	}

	// Health check
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	// WebSocket — must have upgrade check middleware
	app.Use("/ws", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})
	app.Get("/ws", websocket.New(func(c *websocket.Conn) {
		hub.Register(c, mgr)
		defer hub.Unregister(c)
		for {
			if _, _, err := c.ReadMessage(); err != nil {
				break
			}
		}
	}))

	// API routes
	api.RegisterRoutes(app)

	// Static files — use embedded FS by default, filesystem if STATIC_DIR is set
	if os.Getenv("STATIC_DIR") != "" {
		app.Static("/", cfg.StaticDir, fiber.Static{
			Compress: true,
			Index:    "index.html",
		})
	} else {
		subFS, err := fs.Sub(staticFS, "static")
		if err != nil {
			log.Fatalf("failed to load embedded static files: %v", err)
		}
		app.Use("/", filesystem.New(filesystem.Config{
			Root:         http.FS(subFS),
			Browse:       false,
			Index:        "index.html",
			NotFoundFile: "index.html",
		}))
	}

	// Graceful shutdown
	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
		<-quit
		log.Println("Shutting down...")
		mgr.Shutdown()
		_ = app.Shutdown()
	}()

	addr := fmt.Sprintf(":%s", cfg.Port)
	log.Printf("yt-dlp-web listening on http://0.0.0.0%s", addr)
	if err := app.Listen(addr); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
