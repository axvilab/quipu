package profile

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/axvilab/quipu/internal/fsx"
)

// The shim is a symlink named claude that points at quipu, in a folder of quipu's own put first
// on PATH. Run as claude, quipu picks the profile by folder and starts the real claude. The real
// claude (for the native installer ~/.local/bin/claude, rewritten on every update) is never touched.

// ShimDir is the folder that holds the shim.
func ShimDir(dataDir string) string { return filepath.Join(dataDir, "shim") }

// InstallShim makes dir/claude a symlink to self, replacing an older link.
func InstallShim(dir, self string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	link := filepath.Join(dir, "claude")
	if target, err := os.Readlink(link); err == nil {
		if target == self {
			return nil
		}
		if err := os.Remove(link); err != nil {
			return err
		}
	}
	return os.Symlink(self, link)
}

// RemoveShim removes the shim link if it is there.
func RemoveShim(dir string) error {
	err := os.Remove(filepath.Join(dir, "claude"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// ShimInstalled reports whether dir/claude is a link to self.
func ShimInstalled(dir, self string) bool {
	target, err := os.Readlink(filepath.Join(dir, "claude"))
	return err == nil && target == self
}

// ShimActive reports whether a plain "claude" command, looked up on PATH, runs the shim.
func ShimActive(dir string) bool {
	found, err := exec.LookPath("claude")
	if err != nil {
		return false
	}
	a, err1 := os.Stat(found)
	b, err2 := os.Stat(filepath.Join(dir, "claude"))
	return err1 == nil && err2 == nil && os.SameFile(a, b)
}

// rcMarker is the comment quipu writes above its PATH line, so it can find and remove both.
const rcMarker = "# quipu: plain `claude` picks the profile by folder (quipu profile shim off removes this)"

// RCFile returns the shell's startup file for $SHELL and the lines that put dir first on PATH.
func RCFile(home, shell, dir string) (path, block string) {
	shown := dir
	if rest, ok := strings.CutPrefix(dir, home+string(filepath.Separator)); ok {
		shown = "$HOME/" + filepath.ToSlash(rest)
	}
	switch filepath.Base(shell) {
	case "zsh":
		return filepath.Join(home, ".zshrc"), rcMarker + "\nexport PATH=\"" + shown + ":$PATH\"\n"
	case "fish":
		return filepath.Join(home, ".config", "fish", "config.fish"), rcMarker + "\nfish_add_path --prepend \"" + shown + "\"\n"
	case "bash":
		return filepath.Join(home, ".bashrc"), rcMarker + "\nexport PATH=\"" + shown + ":$PATH\"\n"
	}
	return filepath.Join(home, ".profile"), rcMarker + "\nexport PATH=\"" + shown + ":$PATH\"\n"
}

// WithRCBlock returns the rc file content with the block added at the end (once).
func WithRCBlock(content, block string) string {
	if strings.Contains(content, rcMarker) {
		return content
	}
	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	if content != "" {
		content += "\n"
	}
	return content + block
}

// WithoutRCBlock returns the rc file content without quipu's marker and the line after it.
func WithoutRCBlock(content string) string {
	lines := strings.SplitAfter(content, "\n")
	var out []string
	for i := 0; i < len(lines); i++ {
		if strings.TrimRight(lines[i], "\n") == rcMarker {
			i++ // skip the PATH line too
			if len(out) > 0 && strings.TrimSpace(out[len(out)-1]) == "" {
				out = out[:len(out)-1] // and the blank line written before the block
			}
			continue
		}
		out = append(out, lines[i])
	}
	return strings.Join(out, "")
}

// LoginShell is the user's shell: $SHELL, else the user's line in /etc/passwd (services started
// by systemd have no $SHELL), else zsh on macOS, where it is the default.
func LoginShell() string {
	if s := os.Getenv("SHELL"); s != "" {
		return s
	}
	if b, err := os.ReadFile("/etc/passwd"); err == nil {
		uid := strconv.Itoa(os.Getuid())
		for _, line := range strings.Split(string(b), "\n") {
			f := strings.Split(line, ":")
			if len(f) >= 7 && f[2] == uid {
				return f[6]
			}
		}
	}
	if runtime.GOOS == "darwin" {
		return "/bin/zsh"
	}
	return ""
}

// HasRCBlock reports whether the shell startup file has quipu's PATH line.
func HasRCBlock(path string) bool {
	b, err := os.ReadFile(path)
	return err == nil && strings.Contains(string(b), rcMarker)
}

// SetShim turns the shim on or off: the link, and with rc also the PATH line in the shell's
// startup file, which is written through the journal so it can be undone. It returns the rc file.
func SetShim(on, rc bool, home, dataDir, self string, j *fsx.Journal) (string, error) {
	dir := ShimDir(dataDir)
	rcFile, block := RCFile(home, LoginShell(), dir)
	change := WithoutRCBlock
	state := "off"
	if on {
		if err := InstallShim(dir, self); err != nil {
			return rcFile, err
		}
		change = func(s string) string { return WithRCBlock(s, block) }
		state = "on"
	} else if err := RemoveShim(dir); err != nil {
		return rcFile, err
	}
	if !rc {
		return rcFile, nil
	}
	current, err := os.ReadFile(rcFile)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return rcFile, err
	}
	next := change(string(current))
	if next == string(current) {
		return rcFile, nil
	}
	batch, err := fsx.NewBatch()
	if err != nil {
		return rcFile, err
	}
	_, err = j.Write(rcFile, []byte(next), fsx.Meta{Rule: "shim", Title: "claude shim " + state,
		Params: map[string]string{"state": state, "path": rcFile}, Batch: batch})
	return rcFile, err
}
