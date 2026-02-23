package handler

import (
	"strings"

	"yt-dlp-web/internal/download"
	"yt-dlp-web/internal/params"

	"github.com/gofiber/fiber/v2"
)

// API holds references to the download manager.
type API struct {
	mgr *download.Manager
}

// NewAPI creates the API handler.
func NewAPI(mgr *download.Manager) *API {
	return &API{mgr: mgr}
}

// RegisterRoutes defines all REST endpoints.
func (a *API) RegisterRoutes(app *fiber.App) {
	api := app.Group("/api")

	api.Post("/download", a.submitDownload)
	api.Get("/tasks", a.listTasks)
	api.Post("/tasks/:id/cancel", a.cancelTask)
	api.Post("/tasks/:id/pause", a.pauseTask)
	api.Post("/tasks/:id/resume", a.resumeTask)
	api.Post("/tasks/:id/retry", a.retryTask)
	api.Delete("/tasks/:id", a.deleteTask)
	api.Post("/formats", a.listFormats)
	api.Post("/clear-completed", a.clearCompleted)
	api.Get("/stats", a.stats)
}

func (a *API) submitDownload(c *fiber.Ctx) error {
	var req params.DownloadRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request: " + err.Error()})
	}

	url, args := params.BuildArgs(&req)
	if url == "" {
		return c.Status(400).JSON(fiber.Map{"error": "URL is required"})
	}
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return c.Status(400).JSON(fiber.Map{"error": "URL must start with http:// or https://"})
	}

	task := download.NewTask(url, args)
	a.mgr.Submit(task)
	return c.JSON(fiber.Map{"ok": true, "task": task})
}

func (a *API) listTasks(c *fiber.Ctx) error {
	return c.JSON(a.mgr.List())
}

func (a *API) cancelTask(c *fiber.Ctx) error {
	if err := a.mgr.Cancel(c.Params("id")); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (a *API) pauseTask(c *fiber.Ctx) error {
	if err := a.mgr.Pause(c.Params("id")); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (a *API) resumeTask(c *fiber.Ctx) error {
	if err := a.mgr.Resume(c.Params("id")); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (a *API) retryTask(c *fiber.Ctx) error {
	if err := a.mgr.Retry(c.Params("id")); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (a *API) deleteTask(c *fiber.Ctx) error {
	if err := a.mgr.Delete(c.Params("id")); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (a *API) listFormats(c *fiber.Ctx) error {
	var body struct {
		URL  string `json:"url"`
		Args string `json:"args"`
	}
	if err := c.BodyParser(&body); err != nil || body.URL == "" {
		return c.Status(400).JSON(fiber.Map{"error": "url required"})
	}
	if !strings.HasPrefix(body.URL, "http://") && !strings.HasPrefix(body.URL, "https://") {
		return c.Status(400).JSON(fiber.Map{"error": "URL must start with http:// or https://"})
	}
	extra := params.SplitShell(body.Args)
	out, err := a.mgr.ListFormats(body.URL, extra)
	if err != nil {
		return c.JSON(fiber.Map{"ok": false, "output": out, "error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true, "output": out})
}

func (a *API) clearCompleted(c *fiber.Ctx) error {
	n := a.mgr.ClearCompleted()
	return c.JSON(fiber.Map{"ok": true, "cleared": n})
}

func (a *API) stats(c *fiber.Ctx) error {
	return c.JSON(a.mgr.Stats())
}
