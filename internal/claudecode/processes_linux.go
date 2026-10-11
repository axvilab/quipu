package claudecode

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// clockTicks is USER_HZ, the unit of process start times in /proc; 100 on every Linux quipu runs on.
const clockTicks = 100

func processes() ([]Process, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}
	boot := bootTime()
	var out []Process
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		args, _, err := processInfo(pid)
		if err != nil || len(args) == 0 {
			continue
		}
		cwd, err := os.Readlink(filepath.Join("/proc", e.Name(), "cwd"))
		if err != nil {
			continue // not ours
		}
		out = append(out, Process{PID: pid, Args: args, Cwd: cwd, Started: startTime(pid, boot)})
	}
	return out, nil
}

// processInfo reads a process's command line and parent PID.
func processInfo(pid int) ([]string, int, error) {
	dir := filepath.Join("/proc", strconv.Itoa(pid))
	cmd, err := os.ReadFile(filepath.Join(dir, "cmdline"))
	if err != nil {
		return nil, 0, err
	}
	var args []string
	for _, a := range bytes.Split(bytes.TrimRight(cmd, "\x00"), []byte{0}) {
		args = append(args, string(a))
	}
	fields, err := statFields(dir)
	if err != nil || len(fields) < 2 {
		return args, 0, err
	}
	ppid, _ := strconv.Atoi(fields[1]) // field 4 of stat
	return args, ppid, nil
}

// statFields returns /proc/<pid>/stat after the command name, which may contain spaces: the
// first field returned is the state (field 3 of stat).
func statFields(dir string) ([]string, error) {
	b, err := os.ReadFile(filepath.Join(dir, "stat"))
	if err != nil {
		return nil, err
	}
	s := string(b)
	return strings.Fields(s[strings.LastIndexByte(s, ')')+1:]), nil
}

func startTime(pid int, boot time.Time) time.Time {
	fields, err := statFields(filepath.Join("/proc", strconv.Itoa(pid)))
	if err != nil || len(fields) < 20 || boot.IsZero() {
		return time.Time{}
	}
	ticks, err := strconv.ParseInt(fields[19], 10, 64) // field 22: start time since boot
	if err != nil {
		return time.Time{}
	}
	return boot.Add(time.Duration(ticks) * time.Second / clockTicks)
}

func bootTime() time.Time {
	b, err := os.ReadFile("/proc/stat")
	if err != nil {
		return time.Time{}
	}
	for _, line := range strings.Split(string(b), "\n") {
		if v, ok := strings.CutPrefix(line, "btime "); ok {
			if sec, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64); err == nil {
				return time.Unix(sec, 0)
			}
		}
	}
	return time.Time{}
}
