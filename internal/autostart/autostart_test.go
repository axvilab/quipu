package autostart

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPlans(t *testing.T) {
	home := "/home/alice"
	linux, err := For("linux", home, "/home/alice/.local/bin/quipu", "/home/alice/.config", "/home/alice/.local/share", []byte("<svg/>"))
	if err != nil {
		t.Fatal(err)
	}
	unit := linux.Files["/home/alice/.config/systemd/user/quipu.service"]
	if !strings.Contains(unit, "ExecStart=/home/alice/.local/bin/quipu daemon") || !strings.Contains(unit, "WantedBy=default.target") {
		t.Errorf("unit = %s", unit)
	}
	if linux.Files["/home/alice/.local/share/icons/hicolor/scalable/apps/quipu.svg"] != "<svg/>" {
		t.Error("icon not installed")
	}
	if !strings.Contains(linux.Files["/home/alice/.local/share/applications/quipu.desktop"], "Exec=/home/alice/.local/bin/quipu ui") {
		t.Error("desktop entry does not open the UI")
	}

	mac, err := For("darwin", "/Users/alice", "/Users/alice/.local/bin/quipu", "", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	plist := mac.Files["/Users/alice/Library/LaunchAgents/dev.axvilab.quipu.plist"]
	if !strings.Contains(plist, "<string>/Users/alice/.local/bin/quipu</string><string>daemon</string>") {
		t.Errorf("plist = %s", plist)
	}
	if _, err := For("windows", home, "q", "", "", nil); err == nil {
		t.Error("windows should be unsupported")
	}
}

func TestInstallUninstall(t *testing.T) {
	home := t.TempDir()
	p, _ := For("linux", home, "/opt/quipu", filepath.Join(home, ".config"), filepath.Join(home, ".local", "share"), []byte("<svg/>"))
	manifest := filepath.Join(home, "data", "autostart.json")

	if err := Install(p, manifest, true); err != nil {
		t.Fatal(err)
	}
	for path := range p.Files {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("%s not written: %v", path, err)
		}
	}
	if err := Install(p, manifest, true); err != nil {
		t.Errorf("reinstall over our own files: %v", err)
	}
	if err := Uninstall(p, manifest, true); err != nil {
		t.Fatal(err)
	}
	for path := range p.Files {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("%s still exists after uninstall", path)
		}
	}
	if err := Uninstall(p, manifest, true); err == nil {
		t.Error("uninstall when not installed should fail")
	}

	// A unit someone else wrote is never overwritten.
	foreign := filepath.Join(home, ".config", "systemd", "user", "quipu.service")
	os.MkdirAll(filepath.Dir(foreign), 0o755)
	os.WriteFile(foreign, []byte("mine"), 0o644)
	if err := Install(p, manifest, true); err == nil {
		t.Error("install overwrote a file it did not create")
	}
	if b, _ := os.ReadFile(foreign); string(b) != "mine" {
		t.Errorf("foreign file changed: %q", b)
	}
}
