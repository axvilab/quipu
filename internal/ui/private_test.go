package ui

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPrivateFlag(t *testing.T) {
	for in, want := range map[string]string{
		"google-chrome.desktop":             "--incognito",
		"flatpak run org.chromium.Chromium": "--incognito",
		"brave-browser":                     "--incognito",
		"microsoft-edge-stable":             "--inprivate",
		"opera":                             "--private",
		"flatpak run --command=firefox org.mozilla.firefox": "--private-window",
		"librewolf.desktop": "--private-window",
		"epiphany":          "",
	} {
		if got := privateFlag(in); got != want {
			t.Errorf("privateFlag(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDesktopEntry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "org.mozilla.firefox.desktop")
	os.WriteFile(path, []byte("[Desktop Entry]\nName=Firefox\nExec=/usr/bin/flatpak run --branch=stable org.mozilla.firefox @@u %u @@\n[Desktop Action new-window]\nExec=other\n"), 0o644)
	name, execLine := readDesktopEntry(path)
	args := execArgs(execLine)
	if name != "Firefox" || strings.Join(args, " ") != "/usr/bin/flatpak run --branch=stable org.mozilla.firefox @@u @@" {
		t.Errorf("name %q args %q", name, args)
	}
}

func TestInstalledBrowsers(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("PATH-based lookup is the Linux path")
	}
	dir := t.TempDir()
	for _, name := range []string{"firefox", "google-chrome", "google-chrome-stable", "vivaldi"} {
		os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"), 0o755)
	}
	t.Setenv("PATH", dir) // no xdg-settings here, so there is no default browser

	var got []string
	for _, b := range installedBrowsers() {
		got = append(got, b.Name+"="+strings.Join(b.Args, " "))
	}
	want := "Chrome=google-chrome --incognito|Vivaldi=vivaldi --incognito|Firefox=firefox --private-window"
	if strings.Join(got, "|") != want {
		t.Errorf("browsers = %v, want %s", got, want)
	}
	if openPrivateWith("opera", "https://x") {
		t.Error("a browser that is not installed was opened")
	}
}
