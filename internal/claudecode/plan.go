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

// oauthAccount is the account metadata Claude Code keeps in .claude.json; never tokens.
type oauthAccount struct {
	AccountUUID               string `json:"accountUuid"`
	OrganizationUUID          string `json:"organizationUuid"`
	SeatTier                  string `json:"seatTier"`
	OrganizationType          string `json:"organizationType"`
	OrganizationRateLimitTier string `json:"organizationRateLimitTier"`
	UserRateLimitTier         string `json:"userRateLimitTier"`
}

// account reads the account metadata of configDir; zero when there is none.
func account(home, configDir string) oauthAccount {
	var f struct {
		OAuthAccount oauthAccount `json:"oauthAccount"`
	}
	if b, err := os.ReadFile(accountFile(home, configDir)); err == nil {
		json.Unmarshal(b, &f)
	}
	return f.OAuthAccount
}

// PlanTier says which tier of its plan the account in configDir has, as Claude Code last saw it:
// "team standard", "team tier 1 · 5×", "max 5×", "max 20×". "" when the file says nothing more
// than the plan itself. It reads only account fields, and nothing is written.
func PlanTier(home, configDir string) string {
	a := account(home, configDir)
	if a.SeatTier != "" {
		tier := strings.ReplaceAll(a.SeatTier, "_", " ")
		// Seat names like "tier_1" say little; the seat's own usage limit tells how big it is.
		if m := usageMultiple.FindStringSubmatch(a.UserRateLimitTier); m != nil {
			tier += " · " + m[1] + "×"
		}
		return tier
	}
	m := usageMultiple.FindStringSubmatch(a.UserRateLimitTier)
	if m == nil {
		m = usageMultiple.FindStringSubmatch(a.OrganizationRateLimitTier)
	}
	if m == nil || a.OrganizationType == "" {
		return ""
	}
	return strings.TrimPrefix(a.OrganizationType, "claude_") + " " + m[1] + "×"
}

// syncedFolder is the folder under skills/synced that holds the signed-in account's claude.ai
// skills, named <organization>_<account>. Folders of accounts signed in earlier stay on disk, but
// Claude Code lists only this one. "" when the account is unknown.
func syncedFolder(home, configDir string) string {
	a := account(home, configDir)
	if a.OrganizationUUID == "" || a.AccountUUID == "" {
		return ""
	}
	return a.OrganizationUUID + "_" + a.AccountUUID
}
