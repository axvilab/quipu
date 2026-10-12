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

// A profile that shares the main skills folder (a symlink, see profile.Share) sees every synced
// folder too, and must still count only its own account's.
func TestSyncedSkillsThroughSharedFolder(t *testing.T) {
	home := t.TempDir()
	main := filepath.Join(home, ".claude")
	work := filepath.Join(home, ".claude-work")
	for _, f := range []string{"org1_acc1/pdf", "org2_acc2/docx"} {
		dir := filepath.Join(main, "skills", "synced", f)
		os.MkdirAll(dir, 0o755)
		os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: "+filepath.Base(f)+"\ndescription: d\n---\nBody.\n"), 0o644)
	}
	os.MkdirAll(work, 0o755)
	os.Symlink(filepath.Join(main, "skills"), filepath.Join(work, "skills"))
	os.WriteFile(filepath.Join(work, ".claude.json"), []byte(`{"oauthAccount":{"organizationUuid":"org2","accountUuid":"acc2"}}`), 0o600)
	var got []string
	for _, it := range Instructions(home, work, filepath.Join(home, "src")) {
		if it.Scope == "claude.ai" && it.Load == Always {
			got = append(got, it.Name)
		}
	}
	if len(got) != 1 || got[0] != "docx" {
		t.Errorf("work profile sees %v, want [docx]", got)
	}
}

// A project at the home folder reads ~/.claude as its own .claude: synced skills of other
// accounts must not come in that way either.
func TestSyncedSkillsInHomeProject(t *testing.T) {
	home := t.TempDir()
	cfg := filepath.Join(home, ".claude")
	for _, f := range []string{"org1_acc1/pdf", "org2_acc2/docx"} {
		dir := filepath.Join(cfg, "skills", "synced", f)
		os.MkdirAll(dir, 0o755)
		os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: "+filepath.Base(f)+"\ndescription: d\n---\nBody.\n"), 0o644)
	}
	os.WriteFile(filepath.Join(home, ".claude.json"), []byte(`{"oauthAccount":{"organizationUuid":"org1","accountUuid":"acc1"}}`), 0o600)
	var got []string
	for _, it := range Instructions(home, cfg, home) {
		if it.Kind == "skill" && it.Load == Always {
			got = append(got, it.Name)
		}
	}
	if len(got) != 1 || got[0] != "pdf" {
		t.Errorf("home project sees %v, want [pdf]", got)
	}
}

// A skill only the user runs (disable-model-invocation) is not listed to the model.
func TestSkillOnlyForTheUser(t *testing.T) {
	home := t.TempDir()
	cfg := filepath.Join(home, ".claude")
	dir := filepath.Join(cfg, "skills", "release")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: release\ndescription: Cut a release\ndisable-model-invocation: true\n---\nSteps.\n"), 0o644)
	var always, onDemand int
	for _, it := range Instructions(home, cfg, filepath.Join(home, "src")) {
		if it.Name == "release" && it.Load == Always {
			always++
		}
		if it.Name == "release" && it.Load == OnDemand {
			onDemand++
		}
	}
	if always != 0 || onDemand != 1 {
		t.Errorf("listed %d times, body %d times", always, onDemand)
	}
}
