package claudecode

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func touch(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestConfigDirs(t *testing.T) {
	home := t.TempDir()
	touch(t, filepath.Join(home, ".claude", "projects", "p", "s.jsonl"))
	touch(t, filepath.Join(home, ".claude-work", "projects", ".keep"))
	touch(t, filepath.Join(home, ".claude-empty", "settings.json")) // no projects/
	touch(t, filepath.Join(home, ".claude.json"))                   // a file, not a dir
	touch(t, filepath.Join(home, ".config", "x"))

	got, err := ConfigDirs(home)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{filepath.Join(home, ".claude"), filepath.Join(home, ".claude-work")}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ConfigDirs = %v, want %v", got, want)
	}
}

func TestTranscripts(t *testing.T) {
	cfg := t.TempDir()
	p := filepath.Join(cfg, "projects", "-srv-work-demo")
	touch(t, filepath.Join(p, "s-1.jsonl"))
	touch(t, filepath.Join(p, "s-1", "subagents", "agent-abc.jsonl"))
	touch(t, filepath.Join(p, "s-1", "subagents", "workflows", "wf_1", "journal.jsonl"))
	touch(t, filepath.Join(p, "memory", "MEMORY.md"))

	got, err := Transcripts(cfg)
	if err != nil {
		t.Fatal(err)
	}
	want := []Transcript{
		{Path: filepath.Join(p, "s-1.jsonl"), ProjectDir: "-srv-work-demo", SessionID: "s-1", Kind: KindSession},
		{Path: filepath.Join(p, "s-1", "subagents", "agent-abc.jsonl"), ProjectDir: "-srv-work-demo", SessionID: "s-1", Kind: KindSubagent},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Transcripts =\n%+v\nwant\n%+v", got, want)
	}
}
