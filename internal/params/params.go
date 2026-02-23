package params

import (
	"fmt"
	"log"
	"regexp"
	"strings"
)

// dangerousFlags are yt-dlp flags that can execute arbitrary commands or read/write arbitrary files.
var dangerousFlags = map[string]bool{
	"--exec":                  true,
	"--exec-before-download":  true,
	"--batch-file":            true,
	"--config-location":       true,
	"--config-locations":      true,
	"--cookies":               true,
	"--cookies-from-browser":  true,
	"--download-archive":      true,
	"--print-to-file":         true,
	"--output-na-placeholder": true,
	"--postprocessor-args":    true,
	"--ppa":                   true,
}

// SanitizeArgs removes dangerous flags and their values from an argument list.
// Returns sanitized args and an error if dangerous flags were found.
func SanitizeArgs(args []string) ([]string, error) {
	var clean []string
	var blocked []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		// Check exact match or --flag=value form
		flagName := arg
		if idx := strings.Index(arg, "="); idx > 0 {
			flagName = arg[:idx]
		}
		if dangerousFlags[flagName] {
			blocked = append(blocked, flagName)
			// Skip the next token if it's a separate value (not --flag=value)
			if !strings.Contains(arg, "=") && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
			}
			continue
		}
		clean = append(clean, arg)
	}
	if len(blocked) > 0 {
		return clean, fmt.Errorf("blocked dangerous flags: %s", strings.Join(blocked, ", "))
	}
	return clean, nil
}

// DownloadRequest represents what the frontend sends.
type DownloadRequest struct {
	URL  string `json:"url" form:"url"`
	Args string `json:"args" form:"args"` // raw command args or extra custom args

	// UI-driven options (ignored if Args is set with IsRaw=true)
	IsRaw         bool   `json:"is_raw" form:"is_raw"`
	Format        string `json:"format" form:"format"`
	AudioOnly     bool   `json:"audio_only" form:"audio_only"`
	AudioFormat   string `json:"audio_format" form:"audio_format"`
	EmbedSubs     bool   `json:"embed_subs" form:"embed_subs"`
	SubLangs      string `json:"sub_langs" form:"sub_langs"`
	EmbedThumb    bool   `json:"embed_thumb" form:"embed_thumb"`
	EmbedMeta     bool   `json:"embed_meta" form:"embed_meta"`
	EmbedChapter  bool   `json:"embed_chapter" form:"embed_chapter"`
	SponsorBlock  string `json:"sponsorblock" form:"sponsorblock"` // "mark"|"remove"|""
	Proxy         string `json:"proxy" form:"proxy"`
	RateLimit     string `json:"rate_limit" form:"rate_limit"`
	ConcFrags     string `json:"conc_frags" form:"conc_frags"` // concurrent fragments
	OutputTmpl    string `json:"output_tmpl" form:"output_tmpl"`
	ExtractorArgs string `json:"extractor_args" form:"extractor_args"`
	CookiesFrom   string `json:"cookies_from" form:"cookies_from"`
	Username      string `json:"username" form:"username"`
	Password      string `json:"password" form:"password"`
	NoPlaylist    bool   `json:"no_playlist" form:"no_playlist"`
	PlaylistItems string `json:"playlist_items" form:"playlist_items"`
	WriteSubs     bool   `json:"write_subs" form:"write_subs"`
	WriteThumb    bool   `json:"write_thumb" form:"write_thumb"`
	WriteDesc     bool   `json:"write_desc" form:"write_desc"`
	WriteInfoJson bool   `json:"write_info_json" form:"write_info_json"`
	MergeFormat   string `json:"merge_format" form:"merge_format"`
	RemuxVideo    string `json:"remux_video" form:"remux_video"`
	RecodeVideo   string `json:"recode_video" form:"recode_video"`
	PPArgs        string `json:"pp_args" form:"pp_args"`
	SleepInterval string `json:"sleep_interval" form:"sleep_interval"`
	MaxSleep      string `json:"max_sleep" form:"max_sleep"`
}

var shellRe = regexp.MustCompile(`"([^"]*)"|'([^']*)'|(\S+)`)

// SplitShell splits a shell-like command string into tokens.
func SplitShell(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	matches := shellRe.FindAllStringSubmatch(s, -1)
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		switch {
		case m[1] != "":
			out = append(out, m[1])
		case m[2] != "":
			out = append(out, m[2])
		case m[3] != "":
			out = append(out, m[3])
		}
	}
	return out
}

// BuildArgs converts the request into (url, args).
// For raw mode: parses the command string, extracts the URL from the end.
// For UI mode: maps the form fields to yt-dlp flags.
func BuildArgs(req *DownloadRequest) (string, []string) {
	if req.IsRaw {
		return buildRaw(req)
	}
	return req.URL, buildUI(req)
}

func buildRaw(req *DownloadRequest) (string, []string) {
	tokens := SplitShell(req.Args)

	// Strip leading "yt-dlp" or "yt-dlp.exe"
	if len(tokens) > 0 && (tokens[0] == "yt-dlp" || tokens[0] == "yt-dlp.exe") {
		tokens = tokens[1:]
	}

	// The URL is the last non-flag token (starts with http or doesn't start with -)
	url := req.URL
	if len(tokens) > 0 {
		last := tokens[len(tokens)-1]
		if strings.HasPrefix(last, "http") {
			url = last
			tokens = tokens[:len(tokens)-1]
		}
	}
	if url == "" {
		// scan all tokens for a URL
		for i, tok := range tokens {
			if strings.HasPrefix(tok, "http") {
				url = tok
				tokens = append(tokens[:i], tokens[i+1:]...)
				break
			}
		}
	}

	// Sanitize dangerous flags
	tokens, sanitizeErr := SanitizeArgs(tokens)
	if sanitizeErr != nil {
		log.Printf("[params] raw mode: %v", sanitizeErr)
	}
	return url, tokens
}

func buildUI(req *DownloadRequest) []string {
	var a []string

	if req.Format != "" {
		a = append(a, "--format", req.Format)
	}
	if req.AudioOnly {
		a = append(a, "--extract-audio")
		af := req.AudioFormat
		if af == "" {
			af = "mp3"
		}
		a = append(a, "--audio-format", af)
	}
	if req.EmbedSubs {
		a = append(a, "--embed-subs")
	}
	if req.WriteSubs || req.EmbedSubs {
		a = append(a, "--write-subs", "--write-auto-subs")
		langs := req.SubLangs
		if langs == "" {
			langs = "en,zh-Hans,zh-Hant"
		}
		a = append(a, "--sub-langs", langs)
	}
	if req.EmbedThumb {
		a = append(a, "--embed-thumbnail")
	}
	if req.WriteThumb {
		a = append(a, "--write-thumbnail")
	}
	if req.EmbedMeta {
		a = append(a, "--embed-metadata")
	}
	if req.EmbedChapter {
		a = append(a, "--embed-chapters")
	}
	if req.WriteDesc {
		a = append(a, "--write-description")
	}
	if req.WriteInfoJson {
		a = append(a, "--write-info-json")
	}
	if req.SponsorBlock == "mark" {
		a = append(a, "--sponsorblock-mark", "all")
	} else if req.SponsorBlock == "remove" {
		a = append(a, "--sponsorblock-remove", "all")
	}
	if req.Proxy != "" {
		a = append(a, "--proxy", req.Proxy)
	}
	if req.RateLimit != "" {
		a = append(a, "--limit-rate", req.RateLimit)
	}
	if req.ConcFrags != "" {
		a = append(a, "--concurrent-fragments", req.ConcFrags)
	}
	if req.OutputTmpl != "" {
		// Block path traversal, absolute paths (Unix & Windows), UNC paths, and drive-relative paths
		tmpl := req.OutputTmpl
		hasDrive := len(tmpl) >= 2 && ((tmpl[0] >= 'A' && tmpl[0] <= 'Z') || (tmpl[0] >= 'a' && tmpl[0] <= 'z')) && tmpl[1] == ':'
		if !strings.Contains(tmpl, "..") &&
			!strings.HasPrefix(tmpl, "/") &&
			!strings.HasPrefix(tmpl, "\\") &&
			!strings.Contains(tmpl, ":\\") &&
			!strings.Contains(tmpl, ":/") &&
			!strings.ContainsRune(tmpl, 0) &&
			!hasDrive {
			a = append(a, "-o", tmpl)
		}
	}
	if req.ExtractorArgs != "" {
		a = append(a, "--extractor-args", req.ExtractorArgs)
	}
	// --cookies-from-browser is blocked for security (exposes server browser cookies)
	if req.Username != "" {
		a = append(a, "--username", req.Username)
		pw := req.Password
		if pw == "" {
			pw = "\"\""
		}
		a = append(a, "--password", pw)
	}
	if req.NoPlaylist {
		a = append(a, "--no-playlist")
	}
	if req.PlaylistItems != "" {
		a = append(a, "--playlist-items", req.PlaylistItems)
	}
	if req.MergeFormat != "" {
		a = append(a, "--merge-output-format", req.MergeFormat)
	}
	if req.RemuxVideo != "" {
		a = append(a, "--remux-video", req.RemuxVideo)
	}
	if req.RecodeVideo != "" {
		a = append(a, "--recode-video", req.RecodeVideo)
	}
	// --postprocessor-args is blocked for security (allows arbitrary ffmpeg file I/O)
	if req.SleepInterval != "" {
		a = append(a, "--sleep-interval", req.SleepInterval)
	}
	if req.MaxSleep != "" {
		a = append(a, "--max-sleep-interval", req.MaxSleep)
	}

	// Append any custom extra args (sanitized)
	if req.Args != "" {
		extra := SplitShell(req.Args)
		extra, sanitizeErr := SanitizeArgs(extra)
		if sanitizeErr != nil {
			log.Printf("[params] UI extra args: %v", sanitizeErr)
		}
		a = append(a, extra...)
	}

	return a
}
