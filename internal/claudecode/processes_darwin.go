package claudecode

import (
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// processes lists the user's processes with ps; the folder of each Claude Code session comes from
// lsof (only for sessions, to keep it quick).
func processes() ([]Process, error) {
	out, err := exec.Command("ps", "-x", "-o", "pid=,lstart=,command=").Output()
	if err != nil {
		return nil, err
	}
	var list []Process
	for _, line := range strings.Split(string(out), "\n") {
		f := strings.Fields(line)
		if len(f) < 7 { // pid, 5 fields of lstart, command
			continue
		}
		pid, err := strconv.Atoi(f[0])
		if err != nil {
			continue
		}
		args := f[6:]
		p := Process{PID: pid, Args: args}
		if t, err := time.ParseInLocation("Mon Jan 2 15:04:05 2006", strings.Join(f[1:6], " "), time.Local); err == nil {
			p.Started = t
		}
		if isSession(args) {
			p.Cwd = cwdOf(pid)
		}
		list = append(list, p)
	}
	return list, nil
}

func cwdOf(pid int) string {
	out, err := exec.Command("lsof", "-a", "-p", strconv.Itoa(pid), "-d", "cwd", "-Fn").Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		if p, ok := strings.CutPrefix(line, "n"); ok {
			return p
		}
	}
	return ""
}

func processInfo(pid int) ([]string, int, error) {
	out, err := exec.Command("ps", "-o", "ppid=,command=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return nil, 0, err
	}
	f := strings.Fields(string(out))
	if len(f) < 2 {
		return nil, 0, nil
	}
	ppid, _ := strconv.Atoi(f[0])
	return f[1:], ppid, nil
}
