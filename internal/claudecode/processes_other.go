//go:build !linux && !darwin

package claudecode

func processes() ([]Process, error) { return nil, nil }

func processInfo(pid int) ([]string, int, error) { return nil, 0, nil }
