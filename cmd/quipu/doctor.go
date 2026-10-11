package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/axvilab/quipu/internal/ui"
)

// doctorText is the English line for each check; the UI has its own texts in both languages.
var doctorText = map[string]string{
	"claude.ok":         "Claude Code {version} ({path})",
	"claude.fail":       "Claude Code not found: install it from https://claude.ai/install.sh",
	"installs.warn":     "several different claude programs on PATH: {list}; keep one",
	"login.warn":        "profile {name} is not signed in: quipu profile login {name}",
	"broken-link.fail":  "broken shared link {path} -> {target}",
	"rule-profile.fail": "the rule for {folder} points to profile {name}, which does not exist",
	"rule-folder.warn":  "the rule for {folder} (profile {name}): the folder is gone",
	"shim.ok":           "the claude shim works (line in {rc})",
	"shim.warn":         "the claude shim is installed but not on PATH: quipu profile shim on --rc",
	"shim.info":         "the claude shim is off (quipu profile shim on --rc to turn it on)",
	"daemon.ok":         "the quipu service runs",
	"daemon.warn":       "the quipu service is not running: quipu install",
	"window.ok":         "the quipu window is available",
	"window.info":       "the quipu window is not installed; the browser is used",
	"window.warn":       "the quipu window does not start: WebKitGTK 4.1 is missing",
	"rc-autostart.warn": "Remote Control turns on in every session (remoteControlAtStartup)",
}

func runDoctor(ctx context.Context) error {
	home, dir, st, err := openData()
	if err != nil {
		return err
	}
	defer st.Close()
	srv, err := ui.New(home, filepath.Join(home, ".claude"), "default", dir, version, st)
	if err != nil {
		return err
	}
	attachProfiles(srv, home)
	marks := map[string]string{"ok": "ok  ", "info": "--  ", "warn": "!!  ", "fail": "FAIL"}
	problems := 0
	for _, c := range srv.Doctor(ctx) {
		text := doctorText[c.ID+"."+c.State]
		for k, v := range c.Params {
			text = strings.ReplaceAll(text, "{"+k+"}", v)
		}
		fmt.Printf("%s %s\n", marks[c.State], text)
		if c.State == "warn" || c.State == "fail" {
			problems++
		}
	}
	if problems == 0 {
		fmt.Println("\nAll good.")
	}
	return nil
}
