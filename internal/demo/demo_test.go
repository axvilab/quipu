package demo

import (
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/axvilab/quipu/internal/advise"
	"github.com/axvilab/quipu/internal/claudecode"
)

// The demo exists to show every rule, so each one must fire on it.
func TestDemoTriggersEveryRule(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	if err := Create(home, time.Now()); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(home, ".claude")
	shop := filepath.Join(home, "work", "shop-api")

	in := advise.Input{Home: home, ConfigDir: cfg, Project: shop, Items: claudecode.Instructions(home, cfg, shop)}
	fired := map[string]bool{}
	for _, f := range advise.Run(in, nil) {
		fired[f.Rule] = true
	}
	var missing []string
	for _, r := range advise.Rules {
		if !fired[r.ID] {
			missing = append(missing, r.ID)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("rules that do not fire on the demo: %v", missing)
	}

	ts, err := claudecode.Transcripts(cfg)
	if err != nil || len(ts) != 9 {
		t.Errorf("transcripts = %d (%v), want 9", len(ts), err)
	}
	if err := Create(home, time.Now()); err == nil {
		t.Error("Create into a non-empty dir must fail")
	}
}
