package claudecode

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
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
	// Plugins are the plugins this response used (an MCP tool, a skill or a subagent of theirs),
	// or that the user's slash command right before it belonged to. Sorted, without repeats.
	Plugins []string
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
		ID          string          `json:"id"`
		Model       string          `json:"model"`
		StopReason  string          `json:"stop_reason"`
		Usage       Usage           `json:"usage"`
		Content     json.RawMessage `json:"content"` // blocks for the assistant, often text for the user
		Diagnostics *struct {
			CacheMissReason *CacheMiss `json:"cache_miss_reason"`
		} `json:"diagnostics"`
	} `json:"message"`
}

// block is the part of a content block that tells which plugin it used.
type block struct {
	Type  string `json:"type"`
	Name  string `json:"name"`
	Input struct {
		Skill        string `json:"skill"`
		SubagentType string `json:"subagent_type"`
	} `json:"input"`
}

// pluginOf returns the plugin a tool call belongs to, or "": an MCP tool of a plugin's server
// (mcp__plugin_<plugin>_<server>__<tool>), or a skill or subagent named "<plugin>:<name>".
func pluginOf(b block) string {
	if b.Type != "tool_use" {
		return ""
	}
	if rest, ok := strings.CutPrefix(b.Name, "mcp__plugin_"); ok {
		name, _, _ := strings.Cut(rest, "_")
		return name
	}
	for _, ref := range []string{b.Input.Skill, b.Input.SubagentType} {
		if plugin, _, ok := strings.Cut(ref, ":"); ok && plugin != "" {
			return plugin
		}
	}
	return ""
}

// commandPlugin returns the plugin of a slash command the user typed ("/<plugin>:<name>").
func commandPlugin(content json.RawMessage) string {
	var text string
	if json.Unmarshal(content, &text) != nil {
		return ""
	}
	_, after, ok := strings.Cut(text, "<command-name>/")
	if !ok {
		return ""
	}
	name, _, _ := strings.Cut(after, "<")
	if plugin, _, ok := strings.Cut(name, ":"); ok {
		return plugin
	}
	return ""
}

func addPlugin(list []string, p string) []string {
	if p == "" {
		return list
	}
	i := sort.SearchStrings(list, p)
	if i < len(list) && list[i] == p {
		return list
	}
	return append(list[:i], append([]string{p}, list[i:]...)...)
}

// ReadResponses reads a transcript and returns its assistant API responses in file order.
//
// Claude Code writes one line per content block and repeats the response's usage on each of
// them, so responses are deduplicated by message ID; summing raw lines would double count.
// Malformed lines and unknown line types are skipped, because the format is internal and a
// session may still be writing its last line.
func ReadResponses(r io.Reader) ([]Response, ReadStats, error) {
	var (
		out     []Response
		stats   ReadStats
		index   = map[string]int{} // message ID → position in out
		pending []string           // plugins of a slash command, counted with the next response
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
		if l.Type == "user" && l.Message != nil {
			pending = addPlugin(pending, commandPlugin(l.Message.Content))
			continue
		}
		if l.Type != "assistant" || l.Message == nil || l.Message.ID == "" {
			continue
		}
		if l.Message.Model == "<synthetic>" {
			stats.Synthetic++
			continue
		}
		// Every line of a message carries one content block; take plugins from all of them.
		var blocks []block
		json.Unmarshal(l.Message.Content, &blocks)
		if i, ok := index[l.Message.ID]; ok {
			stats.Repeated++
			for _, b := range blocks {
				out[i].Plugins = addPlugin(out[i].Plugins, pluginOf(b))
			}
			continue
		}
		index[l.Message.ID] = len(out)

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
		for _, p := range pending {
			resp.Plugins = addPlugin(resp.Plugins, p)
		}
		pending = nil
		for _, b := range blocks {
			resp.Plugins = addPlugin(resp.Plugins, pluginOf(b))
		}
		out = append(out, resp)
	}
	if err := sc.Err(); err != nil {
		return out, stats, fmt.Errorf("read transcript: %w", err)
	}
	return out, stats, nil
}
