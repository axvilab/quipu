// Command quipu shows what Claude Code knows about each project and what it costs.
//
// The same binary has two roles, chosen by the name it was invoked under:
//   - "quipu": the management CLI;
//   - "claude": the shim that asks which profile to use and then runs the real claude.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/axvilab/quipu/internal/collect"
	"github.com/axvilab/quipu/internal/store"
)

var version = "dev"

const usage = `usage: quipu <command> [flags]

commands:
  advise     check a project's Claude Code setup and propose fixes
  cache      cache hit rate per project or day, and why the cache missed
  daemon     collect Claude Code transcripts before they are deleted
  map        what Claude Code loads for a project and how much it weighs
  version    print the version
`

func main() {
	if filepath.Base(os.Args[0]) == "claude" {
		fmt.Fprintln(os.Stderr, "quipu: shim mode is not implemented yet")
		os.Exit(1)
	}
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var err error
	switch os.Args[1] {
	case "advise":
		err = runAdvise(ctx, os.Args[2:])
	case "cache":
		err = runCache(ctx, os.Args[2:])
	case "daemon":
		err = runDaemon(ctx, os.Args[2:])
	case "map":
		err = runMap(ctx, os.Args[2:])
	case "version", "--version":
		fmt.Println("quipu", version)
	case "help", "-h", "--help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "quipu: unknown command %q\n\n%s", os.Args[1], usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "quipu:", err)
		os.Exit(1)
	}
}

func runDaemon(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("daemon", flag.ExitOnError)
	once := fs.Bool("once", false, "collect once and exit")
	interval := fs.Duration("interval", 10*time.Minute, "time between collections")
	fs.Parse(args)

	home, st, err := openStore()
	if err != nil {
		return err
	}
	defer st.Close()

	for {
		start := time.Now()
		stats, err := collect.Run(ctx, home, st)
		if ctx.Err() != nil {
			return nil
		}
		fmt.Printf("%s collected %d of %d transcripts (%d responses, %d profiles) in %s\n",
			start.Format(time.DateTime), stats.Collected, stats.Seen, stats.Responses, stats.Profiles,
			time.Since(start).Round(time.Millisecond))
		if err != nil {
			fmt.Fprintf(os.Stderr, "quipu: %d transcripts failed, first error: %v\n", stats.Failed, err)
		}
		if *once {
			return err
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(*interval):
		}
	}
}

// openStore opens the store in the user's data dir.
func openStore() (home string, st *store.Store, err error) {
	home, err = os.UserHomeDir()
	if err != nil {
		return "", nil, err
	}
	dir, err := dataDir(home)
	if err != nil {
		return "", nil, err
	}
	st, err = store.Open(dir)
	return home, st, err
}

// dataDir is where quipu keeps collected data: $XDG_DATA_HOME/quipu or ~/.local/share/quipu on
// Linux, ~/Library/Application Support/quipu on macOS.
func dataDir(home string) (string, error) {
	var dir string
	switch {
	case runtime.GOOS == "darwin":
		dir = filepath.Join(home, "Library", "Application Support", "quipu")
	case os.Getenv("XDG_DATA_HOME") != "":
		dir = filepath.Join(os.Getenv("XDG_DATA_HOME"), "quipu")
	default:
		dir = filepath.Join(home, ".local", "share", "quipu")
	}
	return dir, os.MkdirAll(dir, 0o700)
}
