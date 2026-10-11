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
	fs.Parse(args)

	home, dir, st, err := openData()
	if err != nil {
		return err
	}
	defer st.Close()
	configDir := filepath.Join(home, ".claude")
	if *profile != "default" {
		configDir += "-" + *profile
	}

	srv, err := ui.New(home, configDir, *profile, dir, st)
	if err != nil {
		return err
	}
	return srv.Listen(ctx, *port, func(url string) {
		fmt.Println("quipu UI:", url)
		fmt.Println("Press Ctrl+C to stop.")
		if !*noBrowser {
			if err := openBrowser(url); err != nil {
				fmt.Fprintln(os.Stderr, "quipu: could not open the browser, open the link above:", err)
			}
		}
	})
}

func openBrowser(url string) error {
	name := "xdg-open"
	if runtime.GOOS == "darwin" {
		name = "open"
	}
	return exec.Command(name, url).Start()
}
