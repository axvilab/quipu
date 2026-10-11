package claudecode

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Auth is what `claude auth status --json` reports for a config dir. quipu never reads
// credentials itself.
type Auth struct {
	LoggedIn         bool   `json:"loggedIn"`
	AuthMethod       string `json:"authMethod"`
	Email            string `json:"email"`
	OrgName          string `json:"orgName"`
	SubscriptionType string `json:"subscriptionType"`
}

// ConfigEnv returns the environment entry that selects configDir, or "" for the default
// ~/.claude, which must run without CLAUDE_CONFIG_DIR so that everything relying on the default
// (the unsuffixed keychain item on macOS, IDE integrations) keeps working.
func ConfigEnv(home, configDir string) string {
	if filepath.Clean(configDir) == filepath.Join(home, ".claude") {
		return ""
	}
	return "CLAUDE_CONFIG_DIR=" + configDir
}

// AuthStatus asks the real claude binary for the login state of configDir.
func AuthStatus(ctx context.Context, binary, home, configDir string) (Auth, error) {
	cmd := exec.CommandContext(ctx, binary, "auth", "status", "--json")
	cmd.Env = withConfig(os.Environ(), ConfigEnv(home, configDir))
	out, err := cmd.Output()
	var a Auth
	if jerr := json.Unmarshal(out, &a); jerr != nil {
		if err == nil {
			err = jerr
		}
		return a, err
	}
	return a, nil
}

// withConfig replaces any CLAUDE_CONFIG_DIR in env with entry ("" removes it).
func withConfig(env []string, entry string) []string {
	out := make([]string, 0, len(env)+1)
	for _, e := range env {
		if !strings.HasPrefix(e, "CLAUDE_CONFIG_DIR=") {
			out = append(out, e)
		}
	}
	if entry != "" {
		out = append(out, entry)
	}
	return out
}

// Env returns the environment for running claude with configDir.
func Env(home, configDir string) []string {
	return withConfig(os.Environ(), ConfigEnv(home, configDir))
}

// FindBinary returns the first "claude" on PATH that is not self (quipu itself, which may be
// installed under that name as a shim). Files are compared by identity, not by path.
func FindBinary(self string) (string, error) {
	selfInfo, _ := os.Stat(self)
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if dir == "" {
			continue
		}
		path := filepath.Join(dir, "claude")
		info, err := os.Stat(path)
		if err != nil || info.IsDir() || info.Mode()&0o111 == 0 {
			continue
		}
		if selfInfo != nil && os.SameFile(info, selfInfo) {
			continue
		}
		return path, nil
	}
	return "", errors.New("claude not found on PATH; install Claude Code first")
}
