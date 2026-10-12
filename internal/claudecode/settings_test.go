package claudecode

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHooks(t *testing.T) {
	settings := []byte(`{
  "theme": "dark",
  "hooks": {
    "PostToolUse": [{"matcher": "Write", "hooks": [{"type": "command", "command": "prettier --write"}]}]
  },
  "model": "opus"
}`)
	got, err := WithHook(settings, []string{"UserPromptSubmit", "PostToolUse"}, "/bin/quipu hook claude", "quipu hook claude")
	if err != nil {
		t.Fatal(err)
	}
	s := string(got)
	if !HasHook(got, "quipu hook claude") || !strings.Contains(s, "prettier --write") {
		t.Fatalf("after adding:\n%s", s)
	}
	// Top-level order is kept.
	if !(strings.Index(s, `"theme"`) < strings.Index(s, `"hooks"`) && strings.Index(s, `"hooks"`) < strings.Index(s, `"model"`)) {
		t.Errorf("key order changed:\n%s", s)
	}
	// Adding again does not duplicate.
	again, _ := WithHook(got, []string{"UserPromptSubmit", "PostToolUse"}, "/bin/quipu hook claude", "quipu hook claude")
	if n := strings.Count(string(again), "quipu hook claude"); n != 2 {
		t.Errorf("%d entries after adding twice, want 2", n)
	}

	removed, err := WithoutHook(again, "quipu hook claude")
	if err != nil {
		t.Fatal(err)
	}
	if HasHook(removed, "quipu hook claude") || !strings.Contains(string(removed), "prettier --write") {
		t.Errorf("after removing:\n%s", removed)
	}

	// With only our hooks, removing them removes the hooks key altogether.
	only, _ := WithHook([]byte(`{"theme": "dark"}`), []string{"Stop"}, "/bin/quipu hook claude", "quipu hook claude")
	clean, _ := WithoutHook(only, "quipu hook claude")
	if strings.Contains(string(clean), "hooks") {
		t.Errorf("empty hooks left: %s", clean)
	}
}

func TestFindBinaryOutsidePath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", "")
	bin := filepath.Join(home, ".local", "bin", "claude")
	os.MkdirAll(filepath.Dir(bin), 0o755)
	os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755)

	got, err := FindBinary("")
	if err != nil || got != bin {
		t.Fatalf("FindBinary = %q, %v; want %q", got, err, bin)
	}
	// quipu itself under that name (the shim) is skipped.
	if got, err := FindBinary(bin); err == nil && got == bin {
		t.Errorf("FindBinary returned self: %q", got)
	}
}
