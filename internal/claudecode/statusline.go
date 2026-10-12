package claudecode

import (
	"encoding/json"
	"io"
)

// StatusInput is what Claude Code passes on stdin to a statusLine command: the session's own
// numbers, for the account it runs under. Every part may be missing early in a session, and the
// rate limits come only for Pro and Max subscriptions.
type StatusInput struct {
	Model struct {
		DisplayName string `json:"display_name"`
	} `json:"model"`
	Workspace struct {
		CurrentDir string `json:"current_dir"`
	} `json:"workspace"`
	Effort *struct {
		Level string `json:"level"` // low, medium, high, xhigh or max
	} `json:"effort"`
	FastMode bool `json:"fast_mode"`
	Agent    *struct {
		Name string `json:"name"`
	} `json:"agent"`
	PR *struct {
		Number      int    `json:"number"`
		ReviewState string `json:"review_state"` // approved, pending, changes_requested or draft
		Kind        string `json:"kind"`         // "mr" for a GitLab merge request
	} `json:"pr"`
	Cost struct {
		TotalCostUSD *float64 `json:"total_cost_usd"`
	} `json:"cost"`
	ContextWindow struct {
		UsedPercentage    *float64 `json:"used_percentage"`
		ContextWindowSize int64    `json:"context_window_size"`
		CurrentUsage      *struct {
			InputTokens              int64 `json:"input_tokens"`
			CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
			CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
		} `json:"current_usage"`
	} `json:"context_window"`
	RateLimits struct {
		FiveHour *RateWindow `json:"five_hour"`
		SevenDay *RateWindow `json:"seven_day"`
	} `json:"rate_limits"`
	PromptCache *struct {
		Warm          bool     `json:"warm"`
		ExpiresAt     int64    `json:"expires_at"` // Unix seconds the warm cache lasts until
		HitRatio      *float64 `json:"hit_ratio"`
		LastMissAt    int64    `json:"last_miss_at"`
		LastMissCause *struct {
			Causes []string `json:"causes"`
		} `json:"last_miss_cause"`
	} `json:"prompt_cache"`
}

// ContextTokens is what the context window holds now: the input of the last request, as Claude
// Code counts its used_percentage. Zero before the first request.
func (s StatusInput) ContextTokens() int64 {
	u := s.ContextWindow.CurrentUsage
	if u == nil {
		return 0
	}
	return u.InputTokens + u.CacheCreationInputTokens + u.CacheReadInputTokens
}

// RateWindow is one usage limit window.
type RateWindow struct {
	UsedPercentage float64 `json:"used_percentage"`
	ResetsAt       int64   `json:"resets_at"` // Unix seconds
}

// ReadStatusInput parses the status line input; unknown or missing fields are left empty.
func ReadStatusInput(r io.Reader) (StatusInput, error) {
	var in StatusInput
	err := json.NewDecoder(r).Decode(&in)
	return in, err
}

// StatusLineCommand is the settings value that makes Claude Code run command for the status line,
// again every refresh seconds as well as after each reply, so time-based parts (how long the cache
// stays warm) keep counting while the session is idle; 0 runs it only on events.
func StatusLineCommand(command string, refresh int) json.RawMessage {
	v := map[string]any{"type": "command", "command": command}
	if refresh > 0 {
		v["refreshInterval"] = refresh
	}
	b, _ := json.Marshal(v)
	return b
}
