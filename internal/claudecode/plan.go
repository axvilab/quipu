package claudecode

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// accountFile is the file where Claude Code keeps the signed-in account's metadata (never the
// tokens): ~/.claude.json for the default config dir, <dir>/.claude.json for any other.
func accountFile(home, configDir string) string {
	if filepath.Clean(configDir) == filepath.Join(home, ".claude") {
		return filepath.Join(home, ".claude.json")
	}
	return filepath.Join(configDir, ".claude.json")
}

var usageMultiple = regexp.MustCompile(`_(\d+)x$`)

// PlanTier says which tier of its plan the account in configDir has, as Claude Code last saw it:
// "team standard", "team premium", "max 5×", "max 20×". "" when the file says nothing more than
// the plan itself. It reads only these fields, and nothing is written.
func PlanTier(home, configDir string) string {
	b, err := os.ReadFile(accountFile(home, configDir))
	if err != nil {
		return ""
	}
	var f struct {
		OAuthAccount struct {
			SeatTier                  string `json:"seatTier"`
			OrganizationType          string `json:"organizationType"`
			OrganizationRateLimitTier string `json:"organizationRateLimitTier"`
		} `json:"oauthAccount"`
	}
	if json.Unmarshal(b, &f) != nil {
		return ""
	}
	a := f.OAuthAccount
	if a.SeatTier != "" {
		return strings.ReplaceAll(a.SeatTier, "_", " ")
	}
	m := usageMultiple.FindStringSubmatch(a.OrganizationRateLimitTier)
	if m == nil || a.OrganizationType == "" {
		return ""
	}
	return strings.TrimPrefix(a.OrganizationType, "claude_") + " " + m[1] + "×"
}
