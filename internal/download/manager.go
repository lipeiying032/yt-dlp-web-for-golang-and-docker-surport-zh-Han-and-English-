package download

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"yt-dlp-web/internal/config"
)

// Broadcaster is a callback to push task updates to connected WS clients.
type Broadcaster func(t *Task)

// Manager owns the task map, worker pool, and yt-dlp execution.
type Manager struct {
	tasks    map[string]*Task
	order    []string // insertion order for stable listing
	mu       sync.RWMutex
	queue    chan string // task IDs
	cfg      *config.Config
	bc       Broadcaster
	done     chan struct{} // closed on Shutdown
	shutdown sync.Once
}

// NewManager creates the manager and starts worker goroutines.
func NewManager(cfg *config.Config, bc Broadcaster) *Manager {
	m := &Manager{
		tasks: make(map[string]*Task),
		order: make([]string, 0),
		queue: make(chan string, 512),
		cfg:   cfg,
		bc:    bc,
		done:  make(chan struct{}),
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
	if !m.sendQueue(t.ID) {
		m.failTask(t, fmt.Errorf("queue full or shutting down, try again later"))
	}
}

// List returns all tasks sorted newest-first.
func (m *Manager) List() []map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]map[string]interface{}, 0, len(m.order))
	for i := len(m.order) - 1; i >= 0; i-- {
		if t, ok := m.tasks[m.order[i]]; ok {
			out = append(out, t.Snapshot())
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
		t.mu.Unlock()
		return fmt.Errorf("cannot cancel task in state %s", t.Status)
	}
	t.UpdatedAt = time.Now()
	t.mu.Unlock()
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
	if t.Status != StatusRunning {
		t.mu.Unlock()
		return fmt.Errorf("not running")
	}
	if t.cancel != nil {
		t.cancel()
	}
	t.Status = StatusPaused
	t.UpdatedAt = time.Now()
	t.mu.Unlock()
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
	if t.Status != StatusPaused && t.Status != StatusFailed {
		t.mu.Unlock()
		return fmt.Errorf("not paused/failed")
	}
	t.Status = StatusQueued
	t.Error = ""
	t.UpdatedAt = time.Now()
	t.mu.Unlock()
	m.broadcast(t)
	if !m.sendQueue(t.ID) {
		m.failTask(t, fmt.Errorf("queue full or shutting down, try again later"))
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
	if t.Status != StatusFailed && t.Status != StatusCompleted && t.Status != StatusCancelled {
		t.mu.Unlock()
		return fmt.Errorf("cannot retry task in state %s", t.Status)
	}
	t.Status = StatusQueued
	t.Progress = "0%"
	t.Percent = 0
	t.Speed = ""
	t.ETA = ""
	t.Error = ""
	t.Logs = t.Logs[:0]
	t.UpdatedAt = time.Now()
	t.mu.Unlock()
	m.broadcast(t)
	if !m.sendQueue(t.ID) {
		m.failTask(t, fmt.Errorf("queue full or shutting down, try again later"))
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

	m.removeTaskFiles(t.Filename)
	return nil
}

// ClearCompleted removes all completed/failed/cancelled tasks.
func (m *Manager) ClearCompleted() int {
	m.mu.Lock()
	var toDelete []*Task
	count := 0
	for id, t := range m.tasks {
		t.mu.Lock()
		done := t.Status == StatusCompleted || t.Status == StatusFailed || t.Status == StatusCancelled
		t.mu.Unlock()
		if done {
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
		m.removeTaskFiles(t.Filename)
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
	os.MkdirAll(filepath.Join(m.cfg.ConfigDir, "cache"), 0o755)
	cmd := exec.CommandContext(ctx, m.cfg.YtDlpPath, args...)
	cmd.Dir = m.cfg.DownloadDir
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

// removeTaskFiles safely removes a task's files and related artifacts.
// It validates that the file is within the download directory to prevent path traversal.
func (m *Manager) removeTaskFiles(filename string) {
	if filename == "" {
		return
	}
	// Resolve to absolute and verify it's inside the download directory
	absFile, err := filepath.Abs(filename)
	if err != nil {
		return
	}
	absDir, err := filepath.Abs(m.cfg.DownloadDir)
	if err != nil {
		return
	}
	if !strings.HasPrefix(absFile, absDir+string(filepath.Separator)) {
		return
	}

	_ = os.Remove(filename)
	_ = os.Remove(filename + ".part")
	_ = os.Remove(filename + ".ytdl")

	// Remove sibling artifacts using escaped glob pattern
	base := filename
	if ext := filepath.Ext(base); ext != "" {
		base = base[:len(base)-len(ext)]
	}
	// Escape glob special chars in the base name
	escaped := strings.NewReplacer("[", "\\[", "]", "\\]", "?", "\\?", "*", "\\*").Replace(base)
	if matches, _ := filepath.Glob(escaped + "*"); len(matches) > 0 {
		for _, match := range matches {
			absMatch, e := filepath.Abs(match)
			if e != nil || !strings.HasPrefix(absMatch, absDir+string(filepath.Separator)) {
				continue
			}
			_ = os.Remove(match)
		}
	}
}

// sendQueue safely sends a task ID to the queue, returning false if shutdown.
func (m *Manager) sendQueue(id string) bool {
	select {
	case <-m.done:
		return false
	case m.queue <- id:
		return true
	default:
		return false
	}
}

// Shutdown cancels all running tasks and closes the queue.
func (m *Manager) Shutdown() {
	m.shutdown.Do(func() {
		close(m.done)
		m.mu.RLock()
		for _, t := range m.tasks {
			t.mu.Lock()
			if t.cancel != nil {
				t.cancel()
			}
			t.mu.Unlock()
		}
		m.mu.RUnlock()
		close(m.queue)
	})
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
	// If user specified -o, skip the default -o from DefaultArgs
	hasUserOutput := false
	for _, a := range t.Args {
		if a == "-o" || a == "--output" {
			hasUserOutput = true
			break
		}
	}
	args := make([]string, 0, len(m.cfg.DefaultArgs)+len(t.Args)+1)
	for i := 0; i < len(m.cfg.DefaultArgs); i++ {
		if m.cfg.DefaultArgs[i] == "-o" && hasUserOutput && i+1 < len(m.cfg.DefaultArgs) {
			i++ // skip -o and its value
			continue
		}
		args = append(args, m.cfg.DefaultArgs[i])
	}
	args = append(args, t.Args...)
	args = append(args, t.URL)

	// Check yt-dlp exists: stat first, then PATH lookup
	if strings.HasPrefix(m.cfg.YtDlpPath, "NOT_FOUND|") {
		m.failTask(t, fmt.Errorf("YT-DLP NOT FOUND!\n\n%s", m.cfg.YtDlpPath))
		return
	}
	if _, err := os.Stat(m.cfg.YtDlpPath); os.IsNotExist(err) {
		if _, lookErr := exec.LookPath(m.cfg.YtDlpPath); lookErr != nil {
			m.failTask(t, fmt.Errorf("yt-dlp not found at %s or in PATH", m.cfg.YtDlpPath))
			return
		}
	}

	// Log yt-dlp path for debugging
	log.Printf("[execute] YtDlpPath=%s, UsePython=%v", m.cfg.YtDlpPath, m.cfg.UsePython)
	log.Printf("[execute] args=%v", args)

	// Ensure download & cache dirs exist before every execution.
	// On Android the dirs may vanish after startup (storage cleanup, permission changes).
	os.MkdirAll(m.cfg.DownloadDir, 0o755)
	os.MkdirAll(filepath.Join(m.cfg.ConfigDir, "cache"), 0o755)

	var cmd *exec.Cmd
	if m.cfg.UsePython {
		// Python mode: find python3 in the same directory as the script
		scriptDir := filepath.Dir(m.cfg.YtDlpPath)
		pythonPath := filepath.Join(scriptDir, "python3")
		if _, err := os.Stat(pythonPath); os.IsNotExist(err) {
			// Try parent directories
			pythonPath = filepath.Join(scriptDir, "..", "python3")
		}
		log.Printf("[execute] Using Python: %s %s", pythonPath, m.cfg.YtDlpPath)
		cmd = exec.CommandContext(ctx, pythonPath, append([]string{m.cfg.YtDlpPath}, args...)...)
	} else {
		cmd = exec.CommandContext(ctx, m.cfg.YtDlpPath, args...)
	}
	cmd.Dir = m.cfg.DownloadDir // yt-dlp resolves relative -o paths from cwd
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
		// If YtDlpPath is a diagnostic string, show it instead of system error
		if strings.HasPrefix(m.cfg.YtDlpPath, "NOT_FOUND|") {
			m.failTask(t, fmt.Errorf("YT-DLP NOT FOUND!\n\nDiagnostic info:\n%s", m.cfg.YtDlpPath))
		} else {
			m.failTask(t, err)
		}
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
		t.mu.Lock()
		changed := ParseLine(line, t)
		t.mu.Unlock()
		if changed {
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
		t.mu.Lock()
		counts[string(t.Status)]++
		t.mu.Unlock()
	}
	return counts
}
