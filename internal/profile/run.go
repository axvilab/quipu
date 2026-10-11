package profile

import (
	"errors"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

// nonInteractive lists claude arguments and subcommands that never need a person, so quipu must
// not ask which profile to use for them.
var nonInteractive = map[string]bool{
	"-p": true, "--print": true, "-v": true, "--version": true, "-h": true, "--help": true,
	"mcp": true, "plugin": true, "plugins": true, "update": true, "doctor": true, "auth": true,
	"config": true, "install": true, "setup-token": true,
}

// Interactive reports whether claude with args is an interactive session.
func Interactive(args []string) bool {
	for i, a := range args {
		if nonInteractive[a] && (i == 0 || a[0] == '-') {
			return false
		}
	}
	return true
}

// Run starts binary with args and env as a child attached to this terminal and returns its exit
// code. Ctrl+C and window resizes reach the child directly (same process group), so quipu only
// ignores them; SIGTERM and SIGHUP sent to quipu are passed on.
func Run(binary string, args, env []string) (int, error) {
	cmd := exec.Command(binary, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	cmd.Env = env
	if err := cmd.Start(); err != nil {
		return 1, err
	}

	sigs := make(chan os.Signal, 4)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT)
	defer signal.Stop(sigs)
	go func() {
		for s := range sigs {
			if s == syscall.SIGTERM || s == syscall.SIGHUP {
				cmd.Process.Signal(s)
			}
		}
	}()

	err := cmd.Wait()
	var exit *exec.ExitError
	switch {
	case err == nil:
		return 0, nil
	case errors.As(err, &exit):
		if st, ok := exit.Sys().(syscall.WaitStatus); ok && st.Signaled() {
			return 128 + int(st.Signal()), nil
		}
		return exit.ExitCode(), nil
	}
	return 1, err
}
