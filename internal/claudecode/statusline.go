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
	ContextWindow struct {
		UsedPercentage *float64 `json:"used_percentage"`
	} `json:"context_window"`
	RateLimits struct {
		FiveHour *RateWindow `json:"five_hour"`
		SevenDay *RateWindow `json:"seven_day"`
	} `json:"rate_limits"`
	PromptCache *struct {
		HitRatio *float64 `json:"hit_ratio"`
	} `json:"prompt_cache"`
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

// StatusLineCommand is the settings value that makes Claude Code run command for the status line.
func StatusLineCommand(command string) json.RawMessage {
	b, _ := json.Marshal(map[string]string{"type": "command", "command": command})
	return b
}
