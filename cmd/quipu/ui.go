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
	noBrowser := fs.Bool("no-browser", false, "print the link instead of opening a window")
	browser := fs.Bool("browser", false, "open in the browser instead of the quipu window")
	tab := fs.Bool("tab", false, "open a normal browser tab instead of an app window")
	fs.Parse(args)
	useWindow := !*browser && !*tab

	home, dir, st, err := openData()
	if err != nil {
		return err
	}
	defer st.Close()

	// A running daemon already serves the UI: open it instead of starting a second server.
	if url, ok := ui.Running(dir); ok && *profile == "default" && *port == 0 {
		fmt.Println("quipu UI:", ui.Friendly(url))
		if *noBrowser {
			return nil
		}
		if useWindow {
			if w := startWindow(url); w != nil {
				return w.Process.Release()
			}
		}
		return openBrowser(ui.Friendly(url), *tab)
	}
	configDir := filepath.Join(home, ".claude")
	if *profile != "default" {
		configDir += "-" + *profile
	}

	srv, err := ui.New(home, configDir, *profile, dir, version, st)
	if err != nil {
		return err
	}
	attachProfiles(srv, home)
	defer ui.RemoveRuntime(dir)
	ctx, stop := context.WithCancel(ctx)
	defer stop()
	return srv.Listen(ctx, *port, func(url string) {
		ui.WriteRuntime(dir, url)
		fmt.Println("quipu UI:", ui.Friendly(url))
		if *noBrowser {
			fmt.Println("Press Ctrl+C to stop.")
			return
		}
		// This server lives as long as the window: closing the window stops it.
		if useWindow {
			if w := startWindow(url); w != nil {
				go func() { w.Wait(); stop() }()
				return
			}
		}
		fmt.Println("Press Ctrl+C to stop.")
		if err := openBrowser(ui.Friendly(url), *tab); err != nil {
			fmt.Fprintln(os.Stderr, "quipu: could not open the browser, open the link above:", err)
		}
	})
}

// startWindow starts quipu-app, the native window (ADR 0007), on the server at url. It returns nil
// when quipu-app is not installed next to quipu or cannot run here (no GTK or WebKit on Linux),
// so the caller falls back to the browser.
func startWindow(url string) *exec.Cmd {
	app := windowBinary()
	if app == "" || exec.Command(app, "-version").Run() != nil {
		return nil
	}
	cmd := exec.Command(app)
	cmd.Env = append(os.Environ(), "QUIPU_UI_URL="+url)
	if cmd.Start() != nil {
		return nil
	}
	return cmd
}

// windowBinary finds quipu-app: inside Quipu.app on macOS (run from the bundle, it gets the app's
// name and Dock icon), otherwise next to quipu.
func windowBinary() string {
	var candidates []string
	if runtime.GOOS == "darwin" {
		if home, err := os.UserHomeDir(); err == nil {
			candidates = append(candidates, filepath.Join(home, "Applications", "Quipu.app", "Contents", "MacOS", "quipu-app"))
		}
		candidates = append(candidates, "/Applications/Quipu.app/Contents/MacOS/quipu-app")
	}
	if self, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(self), "quipu-app"))
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
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
