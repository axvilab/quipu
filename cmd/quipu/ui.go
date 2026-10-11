package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/axvilab/quipu/internal/ui"
)

func runUI(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("ui", flag.ExitOnError)
	port := fs.Int("port", 0, "port on 127.0.0.1 (0 picks a free one)")
	profile := fs.String("profile", "default", "Claude Code profile: default (~/.claude) or <name> (~/.claude-<name>)")
	noBrowser := fs.Bool("no-browser", false, "print the link instead of opening the browser")
	tab := fs.Bool("tab", false, "open a normal browser tab instead of an app window")
	fs.Parse(args)

	home, dir, st, err := openData()
	if err != nil {
		return err
	}
	defer st.Close()

	// A running daemon already serves the UI: open it instead of starting a second server.
	if url, ok := ui.Running(dir); ok && *profile == "default" && *port == 0 {
		url = ui.Friendly(url)
		fmt.Println("quipu UI:", url)
		if *noBrowser {
			return nil
		}
		return openBrowser(url, *tab)
	}
	configDir := filepath.Join(home, ".claude")
	if *profile != "default" {
		configDir += "-" + *profile
	}

	srv, err := ui.New(home, configDir, *profile, dir, st)
	if err != nil {
		return err
	}
	attachProfiles(srv, home)
	defer ui.RemoveRuntime(dir)
	return srv.Listen(ctx, *port, func(url string) {
		ui.WriteRuntime(dir, url)
		url = ui.Friendly(url)
		fmt.Println("quipu UI:", url)
		fmt.Println("Press Ctrl+C to stop.")
		if !*noBrowser {
			if err := openBrowser(url, *tab); err != nil {
				fmt.Fprintln(os.Stderr, "quipu: could not open the browser, open the link above:", err)
			}
		}
	})
}

// openBrowser opens url in its own app window (Chrome, Chromium, Edge or Brave with --app) so
// quipu looks like an application, and falls back to a normal browser tab.
func openBrowser(url string, tab bool) error {
	if runtime.GOOS == "darwin" {
		if !tab {
			for _, app := range []string{"Google Chrome", "Chromium", "Microsoft Edge", "Brave Browser"} {
				if _, err := os.Stat("/Applications/" + app + ".app"); err == nil {
					return exec.Command("open", "-na", app, "--args", "--app="+url).Start()
				}
			}
		}
		return exec.Command("open", url).Start()
	}
	if !tab {
		for _, name := range []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser", "microsoft-edge", "brave-browser"} {
			if path, err := exec.LookPath(name); err == nil {
				return exec.Command(path, "--app="+url).Start()
			}
		}
	}
	return exec.Command("xdg-open", url).Start()
}

// attachProfiles lets the UI list every profile with its login state.
func attachProfiles(srv *ui.Server, home string) {
	srv.ProfilesFile = filepath.Join(xdgDir("XDG_CONFIG_HOME", home, ".config"), "quipu", "profiles.json")
	if bin, err := realClaude(); err == nil {
		srv.ClaudeBinary = bin
	}
	if self, err := os.Executable(); err == nil {
		srv.SelfBinary = self
	}
}
