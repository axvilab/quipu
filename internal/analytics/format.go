package analytics

import "fmt"

// Human formats a token count as 950, 12.3K, 4.5M or 1.2B.
func Human(n int64) string {
	switch {
	case n >= 1e9:
		return fmt.Sprintf("%.1fB", float64(n)/1e9)
	case n >= 1e6:
		return fmt.Sprintf("%.1fM", float64(n)/1e6)
	case n >= 1e3:
		return fmt.Sprintf("%.1fK", float64(n)/1e3)
	}
	return fmt.Sprint(n)
}

// MissHints explains the cache miss reasons the API reports.
var MissHints = map[string]string{
	"previous_message_not_found": "cache expired: a long pause inside a session",
	"messages_changed":           "earlier conversation content changed",
	"system_changed":             "system prompt changed: CLAUDE.md, settings, output style",
	"tools_changed":              "tool list changed: MCP server or plugin toggled",
	"model_changed":              "model switched mid-session",
}
