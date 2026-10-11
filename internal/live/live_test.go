package live

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSaveAndEnd(t *testing.T) {
	dir := t.TempDir()
	r := Record{SessionID: "abc-123", Cwd: "/src/x", PID: 42, RemoteControl: true, Event: "UserPromptSubmit", Time: time.Now()}
	if err := Save(dir, r); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "abc-123.json")); err != nil {
		t.Fatal(err)
	}
	r.Event = "SessionEnd"
	if err := Save(dir, r); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "abc-123.json")); !os.IsNotExist(err) {
		t.Errorf("record kept after SessionEnd: %v", err)
	}
	// A session id cannot escape the directory.
	if err := Save(dir, Record{SessionID: "../../evil", Event: "Stop"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "______evil.json")); err != nil {
		t.Errorf("unsafe id not sanitized: %v", err)
	}
}

func TestSessionsDropsStaleRecords(t *testing.T) {
	dir := t.TempDir()
	// PID 0 is never a running Claude Code session.
	Save(dir, Record{SessionID: "gone", PID: 0, Event: "Stop", Time: time.Now()})
	if _, err := Sessions(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "gone.json")); !os.IsNotExist(err) {
		t.Errorf("stale record kept: %v", err)
	}
}
