package profile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInteractive(t *testing.T) {
	for args, want := range map[string]bool{
		"":                 true,
		"--model opus":     true,
		"-p hello":         false,
		"--version":        false,
		"mcp list":         false,
		"plugin install x": false,
		"fix the mcp bug":  true, // "mcp" as a word of the prompt, not the subcommand
		"--resume abc":     true,
	} {
		if got := Interactive(strings.Fields(args)); got != want {
			t.Errorf("Interactive(%q) = %v, want %v", args, got, want)
		}
	}
}

func TestRunPassesEnvArgsAndExitCode(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "out")
	fake := filepath.Join(dir, "claude")
	os.WriteFile(fake, []byte("#!/bin/sh\necho \"$CLAUDE_CONFIG_DIR|$*\" > \""+out+"\"\nexit 3\n"), 0o755)

	code, err := Run(fake, []string{"--model", "opus"}, append(os.Environ(), "CLAUDE_CONFIG_DIR=/x/.claude-work"))
	if err != nil {
		t.Fatal(err)
	}
	if code != 3 {
		t.Errorf("exit code = %d, want 3", code)
	}
	if b, _ := os.ReadFile(out); strings.TrimSpace(string(b)) != "/x/.claude-work|--model opus" {
		t.Errorf("child saw %q", b)
	}
}
