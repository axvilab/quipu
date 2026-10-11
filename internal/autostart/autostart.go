// Package autostart sets up quipu to start at login: a systemd user unit on Linux, a launchd
// agent on macOS, plus a menu entry on Linux. Nothing needs root, and everything created is
// recorded so that Uninstall removes exactly that.
package autostart

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Plan is what installing does: files to write and commands to run afterwards.
type Plan struct {
	Files     map[string]string // path → content
	Start     [][]string        // commands that enable and start the service
	Stop      [][]string        // commands that stop and disable it, for uninstall
	ManualRun string            // how to start by hand when commands are skipped
}

const (
	unitName  = "quipu.service"
	agentName = "dev.axvilab.quipu"
)

// For builds the plan for goos ("linux" or "darwin"). binary is the absolute path of quipu,
// configHome is $XDG_CONFIG_HOME or ~/.config, dataHome is $XDG_DATA_HOME or ~/.local/share.
func For(goos, home, binary, configHome, dataHome string, icon []byte) (Plan, error) {
	switch goos {
	case "linux":
		unit := filepath.Join(configHome, "systemd", "user", unitName)
		return Plan{
			Files: map[string]string{
				unit: fmt.Sprintf(`[Unit]
Description=quipu: collect Claude Code transcripts and serve the local web interface

[Service]
ExecStart=%s daemon
Restart=on-failure
RestartSec=30

[Install]
WantedBy=default.target
`, binary),
				filepath.Join(dataHome, "applications", "quipu.desktop"): fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=Quipu
Comment=What Claude Code knows about your projects and what it costs
Exec=%s ui
Icon=quipu
StartupWMClass=quipu
Terminal=false
Categories=Development;
`, binary),
				filepath.Join(dataHome, "icons", "hicolor", "scalable", "apps", "quipu.svg"): string(icon),
			},
			Start:     [][]string{{"systemctl", "--user", "daemon-reload"}, {"systemctl", "--user", "enable", "--now", unitName}},
			Stop:      [][]string{{"systemctl", "--user", "disable", "--now", unitName}},
			ManualRun: "systemctl --user enable --now " + unitName,
		}, nil
	case "darwin":
		plist := filepath.Join(home, "Library", "LaunchAgents", agentName+".plist")
		log := filepath.Join(home, "Library", "Logs", "quipu.log")
		return Plan{
			Files: map[string]string{
				plist: fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key><string>%s</string>
  <key>ProgramArguments</key><array><string>%s</string><string>daemon</string></array>
  <key>RunAtLoad</key><true/>
  <key>KeepAlive</key><dict><key>SuccessfulExit</key><false/></dict>
  <key>StandardOutPath</key><string>%s</string>
  <key>StandardErrorPath</key><string>%s</string>
</dict>
</plist>
`, agentName, xmlEscape(binary), xmlEscape(log), xmlEscape(log)),
			},
			Start:     [][]string{{"launchctl", "load", "-w", plist}},
			Stop:      [][]string{{"launchctl", "unload", "-w", plist}},
			ManualRun: "launchctl load -w " + plist,
		}, nil
	}
	return Plan{}, fmt.Errorf("autostart is not supported on %s", goos)
}

// Install writes the plan's files, records them in manifest and, unless noStart, runs the start
// commands. Existing files that quipu did not create are not overwritten.
func Install(p Plan, manifest string, noStart bool) error {
	created, _ := readManifest(manifest)
	ours := map[string]bool{}
	for _, f := range created {
		ours[f] = true
	}
	for path := range p.Files {
		if _, err := os.Stat(path); err == nil && !ours[path] {
			return fmt.Errorf("%s exists and was not created by quipu; remove it or move it away first", path)
		}
	}
	var paths []string
	for path, content := range p.Files {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return err
		}
		paths = append(paths, path)
	}
	if err := writeManifest(manifest, paths); err != nil {
		return err
	}
	if noStart {
		return nil
	}
	return run(p.Start)
}

// Uninstall stops the service (unless noStop) and removes every file listed in manifest.
func Uninstall(p Plan, manifest string, noStop bool) error {
	paths, err := readManifest(manifest)
	if errors.Is(err, fs.ErrNotExist) {
		return errors.New("quipu autostart is not installed")
	}
	if err != nil {
		return err
	}
	if !noStop {
		run(p.Stop) // keep going: the service may already be stopped
	}
	for _, path := range paths {
		if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	return os.Remove(manifest)
}

func run(cmds [][]string) error {
	for _, c := range cmds {
		out, err := exec.Command(c[0], c[1:]...).CombinedOutput()
		if err != nil {
			return fmt.Errorf("%s: %v: %s", strings.Join(c, " "), err, strings.TrimSpace(string(out)))
		}
	}
	return nil
}

func readManifest(path string) ([]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var paths []string
	return paths, json.Unmarshal(b, &paths)
}

func writeManifest(path string, paths []string) error {
	b, err := json.MarshalIndent(paths, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}

func xmlEscape(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(s)
}
