package profile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestShimLink(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "shim")
	self := filepath.Join(t.TempDir(), "quipu")
	os.WriteFile(self, []byte("#!/bin/sh\n"), 0o755)
	if err := InstallShim(dir, self); err != nil {
		t.Fatal(err)
	}
	if !ShimInstalled(dir, self) {
		t.Fatal("not installed")
	}
	// Moving quipu replaces the link.
	moved := filepath.Join(t.TempDir(), "quipu")
	os.WriteFile(moved, []byte("#!/bin/sh\n"), 0o755)
	if err := InstallShim(dir, moved); err != nil || !ShimInstalled(dir, moved) {
		t.Fatalf("relink: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	if !ShimActive(dir) {
		t.Error("shim first on PATH not seen as active")
	}
	if err := RemoveShim(dir); err != nil || ShimInstalled(dir, moved) {
		t.Errorf("remove: %v", err)
	}
	if err := RemoveShim(dir); err != nil {
		t.Errorf("second remove: %v", err)
	}
}

func TestRCBlock(t *testing.T) {
	home := "/home/u"
	dir := "/home/u/.local/share/quipu/shim"
	path, block := RCFile(home, "/bin/bash", dir)
	if path != "/home/u/.bashrc" || !strings.Contains(block, `export PATH="$HOME/.local/share/quipu/shim:$PATH"`) {
		t.Errorf("bash: %s %q", path, block)
	}
	if p, b := RCFile(home, "/usr/bin/fish", dir); !strings.HasSuffix(p, "config.fish") || !strings.Contains(b, "fish_add_path") {
		t.Errorf("fish: %s %q", p, b)
	}
	before := "alias ll='ls -l'\nexport EDITOR=vim"
	with := WithRCBlock(before, block)
	if WithRCBlock(with, block) != with {
		t.Error("block added twice")
	}
	if got := WithoutRCBlock(with); got != before+"\n" {
		t.Errorf("after removal = %q, want %q", got, before+"\n")
	}
}
