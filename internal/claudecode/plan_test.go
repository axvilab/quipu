package claudecode

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPlanTier(t *testing.T) {
	home := t.TempDir()
	work := filepath.Join(home, ".claude-work")
	os.MkdirAll(work, 0o755)
	tests := []struct {
		name, dir, file, body, want string
	}{
		{"team seat", work, filepath.Join(work, ".claude.json"),
			`{"oauthAccount":{"seatTier":"team_standard","organizationType":"claude_team","organizationRateLimitTier":"default_raven"}}`, "team standard"},
		{"max multiple", filepath.Join(home, ".claude"), filepath.Join(home, ".claude.json"),
			`{"oauthAccount":{"seatTier":null,"organizationType":"claude_max","organizationRateLimitTier":"default_claude_max_5x"}}`, "max 5×"},
		{"plan only", work, filepath.Join(work, ".claude.json"),
			`{"oauthAccount":{"organizationType":"claude_pro","organizationRateLimitTier":"default_claude_ai"}}`, ""},
		{"broken file", work, filepath.Join(work, ".claude.json"), `{`, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.WriteFile(tt.file, []byte(tt.body), 0o600)
			if got := PlanTier(home, tt.dir); got != tt.want {
				t.Errorf("PlanTier = %q, want %q", got, tt.want)
			}
		})
	}
	if got := PlanTier(home, filepath.Join(home, "none")); got != "" {
		t.Errorf("no file = %q", got)
	}
}

func TestSyncedSkillsOfTheCurrentAccount(t *testing.T) {
	home := t.TempDir()
	cfg := filepath.Join(home, ".claude")
	skill := func(folder, name string) {
		dir := filepath.Join(cfg, "skills", "synced", folder, name)
		os.MkdirAll(dir, 0o755)
		os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: "+name+"\ndescription: d\n---\nBody.\n"), 0o644)
	}
	skill("org1_acc1", "pdf")
	skill("org2_acc2", "docx") // left by an account signed in earlier
	names := func() []string {
		var out []string
		for _, it := range Instructions(home, cfg, filepath.Join(home, "src")) {
			if it.Scope == "claude.ai" && it.Load == Always {
				out = append(out, it.Name)
			}
		}
		return out
	}
	// Without account metadata every folder counts, as before.
	if got := names(); len(got) != 2 {
		t.Errorf("unknown account: %v", got)
	}
	os.WriteFile(filepath.Join(home, ".claude.json"), []byte(`{"oauthAccount":{"organizationUuid":"org1","accountUuid":"acc1"}}`), 0o600)
	if got := names(); len(got) != 1 || got[0] != "pdf" {
		t.Errorf("current account: %v", got)
	}
}
