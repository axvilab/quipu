package profile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/axvilab/quipu/internal/fsx"
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

func TestSetRCOtherShell(t *testing.T) {
	home, data := t.TempDir(), t.TempDir()
	j, err := fsx.OpenJournal(filepath.Join(data, "backups"))
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(home, ".zshrc"), []byte("export EDITOR=vim\n"), 0o644)

	rc, err := SetRC(true, "zsh", home, data, j)
	if err != nil || rc != filepath.Join(home, ".zshrc") || !HasRCBlock(rc) {
		t.Fatalf("zsh on: %q %v", rc, err)
	}
	if _, err := SetRC(true, "bash", home, data, j); err != nil || !HasRCBlock(filepath.Join(home, ".bashrc")) {
		t.Fatalf("bash on: %v", err)
	}
	// Turning the shim off removes the line from every shell, keeping the rest of the file.
	t.Setenv("SHELL", "/bin/bash")
	if _, err := SetShim(false, true, home, data, "", j); err != nil {
		t.Fatal(err)
	}
	if HasRCBlock(filepath.Join(home, ".zshrc")) || HasRCBlock(filepath.Join(home, ".bashrc")) {
		t.Error("a PATH line is left after turning the shim off")
	}
	if b, _ := os.ReadFile(filepath.Join(home, ".zshrc")); string(b) != "export EDITOR=vim\n" {
		t.Errorf(".zshrc = %q", b)
	}
}
