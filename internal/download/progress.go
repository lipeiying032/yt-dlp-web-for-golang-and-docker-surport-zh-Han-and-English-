package download

import (
	"regexp"
	"strconv"
	"strings"
)

// yt-dlp --newline output examples:
//   [download]   3.4% of   64.00MiB at    1.23MiB/s ETA 00:50
//   [download]  45.2% of ~  85.49MiB at    2.48MiB/s ETA 00:27 (frag 4/17)
//   [download] 100% of   64.00MiB in 00:01
//   [download] Destination: Video Title [id].webm
//   [youtube] Extracting URL: https://...
//   [info] video_id: Downloading 1 format(s): 248+251
//   [Merger] Merging formats into "file.mkv"
//   [ExtractAudio] Destination: file.mp3

var (
	reProgress = regexp.MustCompile(
		`\[download\]\s+` +
			`([\d.]+)%` +                   // Group 1: percentage
			`\s+of\s+~?\s*([\d.]+\s*\w+)` + // Group 2: total size
			`(?:\s+at\s+([\d.]+\s*\w+/s))?` + // Group 3: speed (optional)
			`(?:\s+ETA\s+([\d:]+))?`)         // Group 4: ETA (optional)

	reComplete = regexp.MustCompile(
		`\[download\]\s+100(?:\.0)?%\s+of\s+~?\s*([\d.]+\s*\w+)\s+in\s+([\d:]+)`)

	reDestination = regexp.MustCompile(
		`\[download\]\s+Destination:\s+(.+)$`)

	reMerger = regexp.MustCompile(
		`\[Merger\]\s+Merging formats into "(.+)"`)

	reAlreadyDl = regexp.MustCompile(
		`\[download\]\s+(.+)\s+has already been downloaded`)
)

// ParseLine inspects a single line of yt-dlp output and updates the task.
// Returns true if a progress-relevant field changed (caller should broadcast).
func ParseLine(line string, t *Task) bool {
	// Progress percentage line
	if m := reProgress.FindStringSubmatch(line); m != nil {
		t.Progress = m[1] + "%"
		if pct, err := strconv.ParseFloat(m[1], 64); err == nil {
			t.Percent = pct
		}
		t.Size = strings.TrimSpace(m[2])
		if m[3] != "" {
			t.Speed = strings.TrimSpace(m[3])
		}
		if m[4] != "" {
			t.ETA = m[4]
		}
		return true
	}

	// 100% completion line
	if m := reComplete.FindStringSubmatch(line); m != nil {
		t.Progress = "100%"
		t.Percent = 100
		t.Size = strings.TrimSpace(m[1])
		t.Speed = ""
		t.ETA = "done"
		return true
	}

	// Destination → extract filename / title
	if m := reDestination.FindStringSubmatch(line); m != nil {
		t.Filename = strings.TrimSpace(m[1])
		if t.Title == t.URL || t.Title == "" {
			t.Title = cleanTitle(t.Filename)
		}
		return true
	}

	// Merger output
	if m := reMerger.FindStringSubmatch(line); m != nil {
		t.Filename = m[1]
		return true
	}

	// Already downloaded
	if m := reAlreadyDl.FindStringSubmatch(line); m != nil {
		t.Progress = "100%"
		t.Percent = 100
		t.Filename = strings.TrimSpace(m[1])
		if t.Title == t.URL || t.Title == "" {
			t.Title = cleanTitle(t.Filename)
		}
		return true
	}

	return false
}

// cleanTitle strips extension and common ID bracketing from a filename.
func cleanTitle(filename string) string {
	// Remove path prefix
	if idx := strings.LastIndexAny(filename, "/\\"); idx >= 0 {
		filename = filename[idx+1:]
	}
	// Remove extension
	if dot := strings.LastIndex(filename, "."); dot > 0 {
		filename = filename[:dot]
	}
	return strings.TrimSpace(filename)
}
