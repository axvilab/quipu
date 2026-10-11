package claudecode

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// maxLineSize bounds a single transcript line. Lines with large tool results can be several MB.
const maxLineSize = 64 << 20

// Usage is the token accounting of one API response.
type Usage struct {
	InputTokens         int64 `json:"input_tokens"`
	CacheReadTokens     int64 `json:"cache_read_input_tokens"`
	CacheCreationTokens int64 `json:"cache_creation_input_tokens"`
	OutputTokens        int64 `json:"output_tokens"`
	CacheCreation       struct {
		Ephemeral5m int64 `json:"ephemeral_5m_input_tokens"`
		Ephemeral1h int64 `json:"ephemeral_1h_input_tokens"`
	} `json:"cache_creation"`
}

// PromptTokens is every input token of the request: uncached, read from cache and written to it.
func (u Usage) PromptTokens() int64 {
	return u.InputTokens + u.CacheReadTokens + u.CacheCreationTokens
}

// CacheMiss is the API's own explanation of why the prompt cache was not used.
type CacheMiss struct {
	Type         string `json:"type"`
	MissedTokens int64  `json:"cache_missed_input_tokens"`
}

// Response is one API response of the assistant, deduplicated across transcript lines.
type Response struct {
	MessageID  string
	RequestID  string
	SessionID  string
	Timestamp  time.Time
	Version    string
	Cwd        string
	GitBranch  string
	Model      string
	StopReason string
	Sidechain  bool
	Usage      Usage
	CacheMiss  *CacheMiss
}

// ReadStats describes what ReadResponses saw besides the responses it returned.
type ReadStats struct {
	Lines     int // non-empty lines
	Malformed int // lines that are not valid JSON (e.g. a truncated last line)
	Repeated  int // assistant lines that repeat an already seen message ID
	Synthetic int // client-generated messages without an API call
}

type line struct {
	Type        string    `json:"type"`
	SessionID   string    `json:"sessionId"`
	Timestamp   time.Time `json:"timestamp"`
	Version     string    `json:"version"`
	Cwd         string    `json:"cwd"`
	GitBranch   string    `json:"gitBranch"`
	IsSidechain bool      `json:"isSidechain"`
	RequestID   string    `json:"requestId"`
	Message     *struct {
		ID          string `json:"id"`
		Model       string `json:"model"`
		StopReason  string `json:"stop_reason"`
		Usage       Usage  `json:"usage"`
		Diagnostics *struct {
			CacheMissReason *CacheMiss `json:"cache_miss_reason"`
		} `json:"diagnostics"`
	} `json:"message"`
}

// ReadResponses reads a transcript and returns its assistant API responses in file order.
//
// Claude Code writes one line per content block and repeats the response's usage on each of
// them, so responses are deduplicated by message ID; summing raw lines would double count.
// Malformed lines and unknown line types are skipped, because the format is internal and a
// session may still be writing its last line.
func ReadResponses(r io.Reader) ([]Response, ReadStats, error) {
	var (
		out   []Response
		stats ReadStats
		seen  = map[string]bool{}
	)
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 1<<20), maxLineSize)
	for sc.Scan() {
		raw := sc.Bytes()
		if len(raw) == 0 {
			continue
		}
		stats.Lines++
		var l line
		if err := json.Unmarshal(raw, &l); err != nil {
			stats.Malformed++
			continue
		}
		if l.Type != "assistant" || l.Message == nil || l.Message.ID == "" {
			continue
		}
		if l.Message.Model == "<synthetic>" {
			stats.Synthetic++
			continue
		}
		if seen[l.Message.ID] {
			stats.Repeated++
			continue
		}
		seen[l.Message.ID] = true

		resp := Response{
			MessageID:  l.Message.ID,
			RequestID:  l.RequestID,
			SessionID:  l.SessionID,
			Timestamp:  l.Timestamp,
			Version:    l.Version,
			Cwd:        l.Cwd,
			GitBranch:  l.GitBranch,
			Model:      l.Message.Model,
			StopReason: l.Message.StopReason,
			Sidechain:  l.IsSidechain,
			Usage:      l.Message.Usage,
		}
		if d := l.Message.Diagnostics; d != nil {
			resp.CacheMiss = d.CacheMissReason
		}
		out = append(out, resp)
	}
	if err := sc.Err(); err != nil {
		return out, stats, fmt.Errorf("read transcript: %w", err)
	}
	return out, stats, nil
}
