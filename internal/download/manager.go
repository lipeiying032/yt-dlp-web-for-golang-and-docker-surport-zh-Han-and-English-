package download

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"yt-dlp-web/internal/config"
)

// Broadcaster is a callback to push task updates to connected WS clients.
type Broadcaster func(t *Task)

// Manager owns the task map, worker pool, and yt-dlp execution.
type Manager struct {
	tasks map[string]*Task
	order []string // insertion order for stable listing
	mu    sync.RWMutex
	queue chan string // task IDs
	cfg   *config.Config
	bc    Broadcaster
}

// NewManager creates the manager and starts worker goroutines.
func NewManager(cfg *config.Config, bc Broadcaster) *Manager {
	m := &Manager{
		tasks: make(map[string]*Task),
		order: make([]string, 0),
		queue: make(chan string, 512),
		cfg:   cfg,
		bc:    bc,
	}
	for i := 0; i < cfg.MaxConcurrent; i++ {
		go m.worker()
	}
	return m
}

// Submit adds a new task to the queue.
func (m *Manager) Submit(t *Task) {
	m.mu.Lock()
	m.tasks[t.ID] = t
	m.order = append(m.order, t.ID)
	m.mu.Unlock()
	m.broadcast(t)
	m.queue <- t.ID
}

// List returns all tasks sorted newest-first.
func (m *Manager) List() []*Task {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*Task, 0, len(m.order))
	for i := len(m.order) - 1; i >= 0; i-- {
		if t, ok := m.tasks[m.order[i]]; ok {
			out = append(out, t)
		}
	}
	return out
}

// Get returns a single task by ID.
func (m *Manager) Get(id string) (*Task, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	t, ok := m.tasks[id]
	return t, ok
}

// Cancel kills the running process or marks queued task as cancelled.
func (m *Manager) Cancel(id string) error {
	t, ok := m.Get(id)
	if !ok {
		return fmt.Errorf("not found")
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	switch t.Status {
	case StatusRunning:
		if t.cancel != nil {
			t.cancel()
		}
		t.Status = StatusCancelled
	case StatusQueued, StatusPaused:
		t.Status = StatusCancelled
		if t.cancel != nil {
			t.cancel()
		}
	default:
		return fmt.Errorf("cannot cancel task in state %s", t.Status)
	}
	t.UpdatedAt = time.Now()
	m.broadcast(t)
	return nil
}

// Pause cancels a running download. Resume will restart it from where yt-dlp left off.
func (m *Manager) Pause(id string) error {
	t, ok := m.Get(id)
	if !ok {
		return fmt.Errorf("not found")
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.Status != StatusRunning {
		return fmt.Errorf("not running")
	}
	if t.cancel != nil {
		t.cancel()
	}
	t.Status = StatusPaused
	t.UpdatedAt = time.Now()
	m.broadcast(t)
	return nil
}

// Resume re-submits a paused or failed task to the queue.
func (m *Manager) Resume(id string) error {
	t, ok := m.Get(id)
	if !ok {
		return fmt.Errorf("not found")
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.Status != StatusPaused && t.Status != StatusFailed {
		return fmt.Errorf("not paused/failed")
	}
	t.Status = StatusQueued
	t.Error = ""
	t.UpdatedAt = time.Now()
	m.broadcast(t)
	// non-blocking send
	select {
	case m.queue <- t.ID:
	default:
	}
	return nil
}

// Retry re-queues a failed or completed task.
func (m *Manager) Retry(id string) error {
	t, ok := m.Get(id)
	if !ok {
		return fmt.Errorf("not found")
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Status = StatusQueued
	t.Progress = "0%"
	t.Percent = 0
	t.Speed = ""
	t.ETA = ""
	t.Error = ""
	t.Logs = t.Logs[:0]
	t.UpdatedAt = time.Now()
	m.broadcast(t)
	select {
	case m.queue <- t.ID:
	default:
	}
	return nil
}

// Delete removes a task from the list and its physical files. Cancels if running.
func (m *Manager) Delete(id string) error {
	_ = m.Cancel(id) // best-effort cancel
	m.mu.Lock()
	t, exists := m.tasks[id]
	if !exists {
		m.mu.Unlock()
		return nil
	}
	delete(m.tasks, id)
	newOrder := make([]string, 0, len(m.order))
	for _, oid := range m.order {
		if oid != id {
			newOrder = append(newOrder, oid)
		}
	}
	m.order = newOrder
	m.mu.Unlock()

	// Best-effort physical file deletion
	if t.Filename != "" {
		_ = os.Remove(t.Filename)
		_ = os.Remove(t.Filename + ".part")
		_ = os.Remove(t.Filename + ".ytdl")

		// Also wipe any sibling artifacts (like info.json, descriptions, thumbnails, subtitles)
		base := t.Filename
		if ext := filepath.Ext(base); ext != "" {
			base = base[:len(base)-len(ext)]
		}
		if matches, _ := filepath.Glob(base + "*"); len(matches) > 0 {
			for _, match := range matches {
				_ = os.Remove(match)
			}
		}
	}
	return nil
}

// ClearCompleted removes all completed/failed/cancelled tasks.
func (m *Manager) ClearCompleted() int {
	m.mu.Lock()
	var toDelete []*Task
	count := 0
	for id, t := range m.tasks {
		if t.Status == StatusCompleted || t.Status == StatusFailed || t.Status == StatusCancelled {
			toDelete = append(toDelete, t)
			delete(m.tasks, id)
			count++
		}
	}
	newOrder := make([]string, 0, len(m.order))
	for _, oid := range m.order {
		if _, ok := m.tasks[oid]; ok {
			newOrder = append(newOrder, oid)
		}
	}
	m.order = newOrder
	m.mu.Unlock()

	// Best-effort physical file deletion for cleared tasks
	for _, t := range toDelete {
		if t.Filename != "" {
			_ = os.Remove(t.Filename)
			_ = os.Remove(t.Filename + ".part")
			_ = os.Remove(t.Filename + ".ytdl")

			base := t.Filename
			if ext := filepath.Ext(base); ext != "" {
				base = base[:len(base)-len(ext)]
			}
			if matches, _ := filepath.Glob(base + "*"); len(matches) > 0 {
				for _, match := range matches {
					_ = os.Remove(match)
				}
			}
		}
	}

	return count
}

// ListFormats runs `yt-dlp -F --no-download URL` and returns the output lines.
func (m *Manager) ListFormats(url string, extraArgs []string) (string, error) {
	args := []string{"--no-colors", "-F", "--no-download"}
	args = append(args, extraArgs...)
	args = append(args, url)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, m.cfg.YtDlpPath, args...)
	cmd.Env = append(os.Environ(),
		"XDG_CACHE_HOME="+m.cfg.ConfigDir+"/cache",
		"XDG_CONFIG_HOME="+m.cfg.ConfigDir,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), err
	}
	return string(out), nil
}

func (m *Manager) broadcast(t *Task) {
	if m.bc != nil {
		m.bc(t)
	}
}

func (m *Manager) worker() {
	for id := range m.queue {
		t, ok := m.Get(id)
		if !ok {
			continue
		}
		t.mu.Lock()
		if t.Status == StatusCancelled {
			t.mu.Unlock()
			continue
		}
		t.mu.Unlock()
		m.execute(t)
	}
}

func (m *Manager) execute(t *Task) {
	ctx, cancel := context.WithCancel(context.Background())
	t.mu.Lock()
	t.Status = StatusRunning
	t.cancel = cancel
	t.UpdatedAt = time.Now()
	t.mu.Unlock()
	defer cancel()
	m.broadcast(t)

	// Build args: defaults + user args + URL
	args := make([]string, 0, len(m.cfg.DefaultArgs)+len(t.Args)+1)
	args = append(args, m.cfg.DefaultArgs...)
	args = append(args, t.Args...)
	args = append(args, t.URL)

	// Check yt-dlp exists: stat first, then PATH lookup
	if _, err := os.Stat(m.cfg.YtDlpPath); os.IsNotExist(err) {
		if _, lookErr := exec.LookPath(m.cfg.YtDlpPath); lookErr != nil {
			m.failTask(t, fmt.Errorf("yt-dlp not found at %s or in PATH", m.cfg.YtDlpPath))
			return
		}
	}

	cmd := exec.CommandContext(ctx, m.cfg.YtDlpPath, args...)
	cmd.Env = append(os.Environ(),
		"XDG_CACHE_HOME="+m.cfg.ConfigDir+"/cache",
		"XDG_CONFIG_HOME="+m.cfg.ConfigDir,
		"HOME="+m.cfg.ConfigDir,
	)

	// Capture stdout and stderr separately, merged into one scanner
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		m.failTask(t, err)
		return
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		m.failTask(t, err)
		return
	}

	if err := cmd.Start(); err != nil {
		m.failTask(t, err)
		return
	}

	t.mu.Lock()
	t.cmd = cmd
	t.mu.Unlock()

	// Merge stdout + stderr
	// Read stdout and stderr concurrently into a combined channel
	lines := make(chan string, 64)
	var wg sync.WaitGroup
	readPipe := func(r io.Reader) {
		defer wg.Done()
		sc := bufio.NewScanner(r)
		sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for sc.Scan() {
			lines <- sc.Text()
		}
	}
	wg.Add(2)
	go readPipe(stdoutPipe)
	go readPipe(stderrPipe)
	go func() { wg.Wait(); close(lines) }()

	for line := range lines {
		t.AddLog(line) // AddLog has its own lock
		if ParseLine(line, t) {
			m.broadcast(t)
		}
	}

	waitErr := cmd.Wait()

	t.mu.Lock()
	if waitErr != nil {
		if ctx.Err() == context.Canceled {
			if t.Status != StatusPaused {
				t.Status = StatusCancelled
			}
		} else {
			t.Status = StatusFailed
			t.Error = waitErr.Error()
		}
	} else {
		t.Status = StatusCompleted
		t.Progress = "100%"
		t.Percent = 100
	}
	t.cmd = nil
	t.cancel = nil
	t.UpdatedAt = time.Now()
	t.mu.Unlock()
	m.broadcast(t)
}

func (m *Manager) failTask(t *Task, err error) {
	t.mu.Lock()
	t.Status = StatusFailed
	t.Error = err.Error()

	// Prevent deadlock! t.AddLog also tries to lock t.mu, so we interact with the slice directly.
	t.Logs = append(t.Logs, "ERROR: "+err.Error())
	if len(t.Logs) > 500 {
		t.Logs = t.Logs[len(t.Logs)-500:]
	}

	t.UpdatedAt = time.Now()
	t.mu.Unlock()
	m.broadcast(t)
}

// Stats returns summary counts.
func (m *Manager) Stats() map[string]int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	counts := map[string]int{"total": len(m.tasks)}
	for _, t := range m.tasks {
		counts[string(t.Status)]++
	}
	return counts
}
