package download

import (
	"context"
	"os/exec"
	"sync"
	"time"

	"crypto/rand"
	"encoding/hex"
)

// TaskStatus represents the lifecycle state of a download task.
type TaskStatus string

const (
	StatusQueued    TaskStatus = "queued"
	StatusRunning   TaskStatus = "running"
	StatusPaused    TaskStatus = "paused"
	StatusCompleted TaskStatus = "completed"
	StatusFailed    TaskStatus = "failed"
	StatusCancelled TaskStatus = "cancelled"
)

// Task is a single download job.
type Task struct {
	ID        string     `json:"id"`
	URL       string     `json:"url"`
	Title     string     `json:"title"`
	Status    TaskStatus `json:"status"`
	Progress  string     `json:"progress"`   // e.g. "45.2%"
	Percent   float64    `json:"percent"`     // 0–100 numeric for progress bar
	Size      string     `json:"size"`
	Speed     string     `json:"speed"`
	ETA       string     `json:"eta"`
	Filename  string     `json:"filename"`
	Error     string     `json:"error,omitempty"`
	Logs      []string   `json:"logs"`
	Args      []string   `json:"args"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`

	// Internal — not serialized
	cmd    *exec.Cmd          `json:"-"`
	cancel context.CancelFunc `json:"-"`
	mu     sync.Mutex         `json:"-"`
}

func randomID() string {
	b := make([]byte, 6)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// NewTask creates a queued task ready for submission.
func NewTask(url string, args []string) *Task {
	return &Task{
		ID:        randomID(),
		URL:       url,
		Title:     url, // will be overwritten when yt-dlp emits metadata
		Status:    StatusQueued,
		Progress:  "0%",
		Percent:   0,
		Args:      args,
		Logs:      make([]string, 0, 64),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// Snapshot returns a JSON-safe copy of the task under lock.
func (t *Task) Snapshot() map[string]interface{} {
	t.mu.Lock()
	defer t.mu.Unlock()
	return map[string]interface{}{
		"id": t.ID, "url": t.URL, "title": t.Title,
		"status": t.Status, "progress": t.Progress, "percent": t.Percent,
		"size": t.Size, "speed": t.Speed, "eta": t.ETA,
		"filename": t.Filename, "error": t.Error, "logs": t.Logs,
		"args": t.Args, "created_at": t.CreatedAt, "updated_at": t.UpdatedAt,
	}
}

// AddLog appends a line, capped at 500 entries to bound memory.
func (t *Task) AddLog(line string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Logs = append(t.Logs, line)
	if len(t.Logs) > 500 {
		t.Logs = t.Logs[len(t.Logs)-500:]
	}
	t.UpdatedAt = time.Now()
}
