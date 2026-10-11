package claudecode

import (
	"path/filepath"
	"strings"
	"time"
)

// Process is a running Claude Code session.
type Process struct {
	PID     int
	Args    []string
	Cwd     string
	Started time.Time
	// RemoteControl is true when the command line turns Remote Control on. A session where it was
	// switched on with /remote-control does not show it here.
	RemoteControl bool
}

// RunningSessions lists the user's running Claude Code sessions. Helper processes started by
// Claude Code (the Chrome bridge, MCP server mode) and short commands are not sessions.
func RunningSessions() ([]Process, error) {
	all, err := processes()
	if err != nil {
		return nil, err
	}
	var out []Process
	for _, p := range all {
		if isSession(p.Args) {
			p.RemoteControl = remoteControlFlag(p.Args)
			out = append(out, p)
		}
	}
	return out, nil
}

// SessionOf returns the PID of the Claude Code session that pid runs under (pid itself or an
// ancestor), or 0. Hooks use it: Claude Code starts them through a shell.
func SessionOf(pid int) int {
	for i := 0; pid > 1 && i < 16; i++ {
		args, ppid, err := processInfo(pid)
		if err != nil {
			return 0
		}
		if isSession(args) {
			return pid
		}
		pid = ppid
	}
	return 0
}

// notSessions are subcommands and flags of claude that run something other than a session.
var notSessions = map[string]bool{
	"mcp": true, "config": true, "doctor": true, "update": true, "install": true, "auth": true,
	"setup-token": true, "plugin": true, "agents": true, "migrate-installer": true,
	"--chrome-native-host": true, "--version": true, "-v": true, "--help": true, "-h": true,
}

// isSession tells a Claude Code session from other processes by its command line: the native
// binary (claude) or the npm package run by node (…/claude-code/cli.js).
func isSession(args []string) bool {
	if len(args) == 0 {
		return false
	}
	rest := args[1:]
	switch base := filepath.Base(args[0]); {
	case base == "claude":
	case strings.HasPrefix(base, "node") && len(args) > 1 && strings.Contains(filepath.ToSlash(args[1]), "claude-code/cli"):
		rest = args[2:]
	default:
		return false
	}
	for _, a := range rest {
		if notSessions[a] {
			return false
		}
	}
	return true
}

func remoteControlFlag(args []string) bool {
	for _, a := range args[1:] {
		if a == "remote-control" || a == "--remote-control" || a == "--rc" {
			return true
		}
	}
	return false
}
