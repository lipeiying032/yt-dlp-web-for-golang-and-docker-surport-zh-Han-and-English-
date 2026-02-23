package params

import (
	"regexp"
	"strings"
)

// DownloadRequest represents what the frontend sends.
type DownloadRequest struct {
	URL  string `json:"url" form:"url"`
	Args string `json:"args" form:"args"` // raw command args or extra custom args

	// UI-driven options (ignored if Args is set with IsRaw=true)
	IsRaw        bool   `json:"is_raw" form:"is_raw"`
	Format       string `json:"format" form:"format"`
	AudioOnly    bool   `json:"audio_only" form:"audio_only"`
	AudioFormat  string `json:"audio_format" form:"audio_format"`
	EmbedSubs    bool   `json:"embed_subs" form:"embed_subs"`
	SubLangs     string `json:"sub_langs" form:"sub_langs"`
	EmbedThumb   bool   `json:"embed_thumb" form:"embed_thumb"`
	EmbedMeta    bool   `json:"embed_meta" form:"embed_meta"`
	EmbedChapter bool   `json:"embed_chapter" form:"embed_chapter"`
	SponsorBlock string `json:"sponsorblock" form:"sponsorblock"` // "mark"|"remove"|""
	Proxy        string `json:"proxy" form:"proxy"`
	RateLimit    string `json:"rate_limit" form:"rate_limit"`
	ConcFrags   string `json:"conc_frags" form:"conc_frags"` // concurrent fragments
	OutputTmpl   string `json:"output_tmpl" form:"output_tmpl"`
	ExtractorArgs string `json:"extractor_args" form:"extractor_args"`
	CookiesFrom  string `json:"cookies_from" form:"cookies_from"`
	Username     string `json:"username" form:"username"`
	Password     string `json:"password" form:"password"`
	NoPlaylist   bool   `json:"no_playlist" form:"no_playlist"`
	PlaylistItems string `json:"playlist_items" form:"playlist_items"`
	WriteSubs    bool   `json:"write_subs" form:"write_subs"`
	WriteThumb   bool   `json:"write_thumb" form:"write_thumb"`
	WriteDesc    bool   `json:"write_desc" form:"write_desc"`
	WriteInfoJson bool  `json:"write_info_json" form:"write_info_json"`
	MergeFormat  string `json:"merge_format" form:"merge_format"`
	RemuxVideo   string `json:"remux_video" form:"remux_video"`
	RecodeVideo  string `json:"recode_video" form:"recode_video"`
	PPArgs       string `json:"pp_args" form:"pp_args"`
	SleepInterval string `json:"sleep_interval" form:"sleep_interval"`
	MaxSleep     string `json:"max_sleep" form:"max_sleep"`
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
		// Block path traversal and absolute paths
		if !strings.Contains(req.OutputTmpl, "..") && !strings.HasPrefix(req.OutputTmpl, "/") && !strings.Contains(req.OutputTmpl, ":\\") {
			a = append(a, "-o", req.OutputTmpl)
		}
	}
	if req.ExtractorArgs != "" {
		a = append(a, "--extractor-args", req.ExtractorArgs)
	}
	if req.CookiesFrom != "" {
		a = append(a, "--cookies-from-browser", req.CookiesFrom)
	}
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
	if req.PPArgs != "" {
		a = append(a, "--postprocessor-args", req.PPArgs)
	}
	if req.SleepInterval != "" {
		a = append(a, "--sleep-interval", req.SleepInterval)
	}
	if req.MaxSleep != "" {
		a = append(a, "--max-sleep-interval", req.MaxSleep)
	}

	// Append any custom extra args
	if req.Args != "" {
		extra := SplitShell(req.Args)
		a = append(a, extra...)
	}

	return a
}
