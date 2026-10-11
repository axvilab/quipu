package claudecode

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ConfigDirs returns the Claude Code config directories under home: the default ~/.claude and
// every ~/.claude-<name> used as CLAUDE_CONFIG_DIR of another profile. Only directories that
// contain a projects/ subdirectory are returned, so unrelated ~/.claude-* files are ignored.
func ConfigDirs(home string) ([]string, error) {
	entries, err := os.ReadDir(home)
	if err != nil {
		return nil, err
	}
	var dirs []string
	for _, e := range entries {
		name := e.Name()
		if name != ".claude" && !strings.HasPrefix(name, ".claude-") {
			continue
		}
		dir := filepath.Join(home, name)
		if fi, err := os.Stat(filepath.Join(dir, "projects")); err == nil && fi.IsDir() {
			dirs = append(dirs, dir)
		}
	}
	sort.Strings(dirs)
	return dirs, nil
}

// TranscriptKind tells a main session transcript from a subagent one.
type TranscriptKind string

const (
	KindSession  TranscriptKind = "session"
	KindSubagent TranscriptKind = "subagent"
)

// Transcript is one JSONL file under <config>/projects.
type Transcript struct {
	Path       string
	ProjectDir string // directory name under projects/, see ProjectDirName
	SessionID  string // the main session the file belongs to
	Kind       TranscriptKind
}

// Transcripts lists session and subagent transcripts of one config directory.
// Workflow journals and other files are not transcripts and are skipped.
func Transcripts(configDir string) ([]Transcript, error) {
	root := filepath.Join(configDir, "projects")
	var out []Transcript

	sessions, err := filepath.Glob(filepath.Join(root, "*", "*.jsonl"))
	if err != nil {
		return nil, err
	}
	for _, p := range sessions {
		out = append(out, Transcript{
			Path:       p,
			ProjectDir: filepath.Base(filepath.Dir(p)),
			SessionID:  strings.TrimSuffix(filepath.Base(p), ".jsonl"),
			Kind:       KindSession,
		})
	}

	agents, err := filepath.Glob(filepath.Join(root, "*", "*", "subagents", "agent-*.jsonl"))
	if err != nil {
		return nil, err
	}
	for _, p := range agents {
		sessionDir := filepath.Dir(filepath.Dir(p))
		out = append(out, Transcript{
			Path:       p,
			ProjectDir: filepath.Base(filepath.Dir(sessionDir)),
			SessionID:  filepath.Base(sessionDir),
			Kind:       KindSubagent,
		})
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

// ProfileName returns the profile a config directory belongs to: "default" for ~/.claude and
// "<name>" for ~/.claude-<name>.
func ProfileName(configDir string) string {
	base := filepath.Base(configDir)
	if name, ok := strings.CutPrefix(base, ".claude-"); ok && name != "" {
		return name
	}
	return "default"
}
