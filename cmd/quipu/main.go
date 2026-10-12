// Command quipu shows what Claude Code knows about each project and what it costs.
//
// The same binary has two roles, chosen by the name it was invoked under:
//   - "quipu": the management CLI;
//   - "claude": the shim that asks which profile to use and then runs the real claude.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/axvilab/quipu/internal/collect"
	"github.com/axvilab/quipu/internal/store"
	"github.com/axvilab/quipu/internal/ui"
)

var version = "dev"

// defaultUIPort is where the daemon serves the web interface, so its address stays the same.
const defaultUIPort = 47800

const usage = `usage: quipu <command> [flags]

commands:
  advise     check a project's Claude Code setup and propose fixes
  backup     save the Claude Code setup of every profile to an archive or a git repository
  cache      cache hit rate per project or day, and why the cache missed
  daemon     collect transcripts in the background and serve the web interface
  demo       create a fake home with a Claude Code setup to try quipu on
  doctor     check Claude Code, profiles, the claude shim and quipu itself
  install    start quipu at login and add it to the application menu
  map        what Claude Code loads for a project and how much it weighs
  new        set up a new or freshly cloned project for Claude Code
  profile    Claude accounts (personal, work…) and which folder uses which
  restore    show what a backup would change, and write it with --apply
  run        start claude under the profile of the current folder
  statusline the status line Claude Code shows: profile, context, limits (set up on the Profiles page)
  ui         open the web interface
  uninstall  undo quipu install
  version    print the version
`

func main() {
	// claude auth login runs quipu as $BROWSER when signing in from the UI.
	if ui.CaptureLoginURL(os.Args) {
		return
	}
	// Run as claude (the shim, see quipu profile shim): pick the profile and run the real claude.
	// Every argument is claude's, so quipu reads none of them as its own flags.
	if filepath.Base(os.Args[0]) == "claude" {
		code, err := runClaudeWith(os.Args[1:], "")
		if err != nil {
			fmt.Fprintln(os.Stderr, "quipu:", err)
		}
		os.Exit(code)
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
	case "backup":
		err = runBackup(ctx, os.Args[2:])
	case "restore":
		err = runRestore(os.Args[2:])
	case "cache":
		err = runCache(ctx, os.Args[2:])
	case "daemon":
		err = runDaemon(ctx, os.Args[2:])
		if errors.Is(err, errRestart) {
			err = restartSelf()
		}
	case "doctor":
		err = runDoctor(ctx)
	case "demo":
		err = runDemo(os.Args[2:])
	case "install":
		err = runInstall(os.Args[2:], false)
	case "uninstall":
		err = runInstall(os.Args[2:], true)
	case "map":
		err = runMap(ctx, os.Args[2:])
	case "new":
		err = runNew(os.Args[2:])
	case "profile", "profiles":
		err = runProfile(ctx, os.Args[2:])
	case "run":
		stop() // the child gets Ctrl+C itself; quipu run must not exit before it
		code, err := runClaude(os.Args[2:])
		if err != nil {
			fmt.Fprintln(os.Stderr, "quipu:", err)
		}
		os.Exit(code)
	case "hook":
		runHook(os.Args[2:]) // never fails: a hook error would bother the Claude Code session
	case "statusline":
		err = runStatusline(os.Stdin, os.Stdout)
	case "ui":
		err = runUI(ctx, os.Args[2:])
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
	noUI := fs.Bool("no-ui", false, "do not serve the web interface")
	uiPort := fs.Int("ui-port", defaultUIPort, "port of the web interface on 127.0.0.1")
	fs.Parse(args)

	home, dir, st, err := openData()
	if err != nil {
		return err
	}
	defer st.Close()

	// An update installed from the web interface stops the daemon, and main starts the new binary.
	ctx, stop := context.WithCancel(ctx)
	defer stop()
	var restart atomic.Bool
	stopped := func() error {
		if restart.Load() {
			return errRestart
		}
		return nil
	}

	if !*once && !*noUI {
		srv, err := ui.New(home, filepath.Join(home, ".claude"), "default", dir, version, st)
		if err != nil {
			return err
		}
		attachProfiles(srv, home)
		srv.Restart = func() { restart.Store(true); stop() }
		srv.UpdateBase = os.Getenv("QUIPU_UPDATE_BASE") // a mirror, or a fake release in tests
		go func() {
			err := srv.Listen(ctx, *uiPort, func(url string) {
				if err := ui.WriteRuntime(dir, url); err != nil {
					fmt.Fprintln(os.Stderr, "quipu:", err)
				}
				fmt.Println("web interface:", strings.SplitN(ui.Friendly(url), "?", 2)[0], "(open it with `quipu ui`)")
			})
			if err != nil {
				fmt.Fprintln(os.Stderr, "quipu: web interface stopped:", err)
			}
		}()
		defer ui.RemoveRuntime(dir)
	}

	for {
		start := time.Now()
		stats, err := collect.Run(ctx, home, st)
		if ctx.Err() != nil {
			return stopped()
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
			return stopped()
		case <-time.After(*interval):
		}
	}
}

// errRestart asks main to start the binary now at quipu's path in place of the daemon.
var errRestart = errors.New("restart")

// restartSelf replaces this process with the binary at quipu's path, with the same arguments. The
// PID stays, so systemd and launchd keep tracking the daemon; the store is closed by then.
func restartSelf() error {
	exe, err := os.Executable() // on Linux, the path even after the old file was replaced
	if err != nil {
		return err
	}
	return syscall.Exec(exe, os.Args, os.Environ())
}

// openStore opens the store in the user's data dir.
func openStore() (home string, st *store.Store, err error) {
	home, _, st, err = openData()
	return home, st, err
}

// openData returns the home dir, quipu's data dir and the store in it.
func openData() (home, dir string, st *store.Store, err error) {
	home, err = os.UserHomeDir()
	if err != nil {
		return "", "", nil, err
	}
	if dir, err = dataDir(home); err != nil {
		return "", "", nil, err
	}
	st, err = store.Open(dir)
	return home, dir, st, err
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
