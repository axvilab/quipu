package backup

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// SaveToGit mirrors the backup into the git working copy dir (initialized if it is not one yet)
// and commits it. It pushes only when push is set, with the user's own git setup; quipu never
// handles git credentials. It reports whether anything changed since the last backup.
func SaveToGit(ctx context.Context, dir string, m Manifest, entries []Entry, push bool) (bool, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return false, err
	}
	if _, err := os.Stat(filepath.Join(dir, ".git")); os.IsNotExist(err) {
		if _, err := git(ctx, dir, "init", "-q"); err != nil {
			return false, err
		}
	}
	if err := WriteDir(dir, m, entries); err != nil {
		return false, err
	}
	// The manifest's time changes on every run; without other changes there is nothing to commit.
	status, err := git(ctx, dir, "status", "--porcelain", "--", "claude", "quipu")
	if err != nil {
		return false, err
	}
	changed := strings.TrimSpace(status) != ""
	if !changed {
		git(ctx, dir, "checkout", "-q", "--", manifestName) // fails harmlessly before the first commit
	} else {
		// Only paths that exist: without quipu profiles there is no quipu/ folder, and git add
		// fails on a path that matches nothing.
		paths := []string{manifestName}
		for _, p := range []string{"claude", "quipu"} {
			if _, err := os.Stat(filepath.Join(dir, p)); err == nil {
				paths = append(paths, p)
			}
		}
		if _, err := git(ctx, dir, append([]string{"add", "-A", "--"}, paths...)...); err != nil {
			return false, err
		}
		args := []string{"commit", "-q", "-m", "quipu backup " + m.Created.Local().Format("2006-01-02 15:04")}
		if who, _ := git(ctx, dir, "config", "user.email"); strings.TrimSpace(who) == "" {
			args = append([]string{"-c", "user.name=quipu", "-c", "user.email=quipu@localhost"}, args...)
		}
		if _, err := git(ctx, dir, args...); err != nil {
			return false, err
		}
	}
	if push {
		// -u origin HEAD also works for the first push into an empty repository.
		if _, err := git(ctx, dir, "push", "-q", "-u", "origin", "HEAD"); err != nil {
			return changed, err
		}
	}
	return changed, nil
}

func git(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0") // never wait for a password prompt
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		return out.String(), fmt.Errorf("git %s: %v: %s", args[0], err, strings.TrimSpace(errb.String()))
	}
	return out.String(), nil
}
