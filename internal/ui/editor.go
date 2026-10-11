package ui

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/axvilab/quipu/internal/analytics"
	"github.com/axvilab/quipu/internal/claudecode"
	"github.com/axvilab/quipu/internal/fsx"
)

// maxEditBytes keeps the editor to instruction files (and under the API's 1 MB body limit).
const maxEditBytes = 512 << 10

// File is an instruction file opened in the editor.
type File struct {
	Path    string `json:"path"` // as shown, with ~
	Name    string `json:"name"`
	Content string `json:"content"`
	Version string `json:"version"` // hash of Content; saving with an old version is refused
	Tokens  int64  `json:"tokens"`
	Always  bool   `json:"always"` // the whole file is sent in every session
	Exists  bool   `json:"exists"`
}

// editable lists the files the editor may open, keyed by absolute path, with whether the whole
// file is sent in every session: what Claude Code loads for the known projects and the profile,
// except files synced from claude.ai or installed by plugins, which are managed elsewhere.
func (s *Service) editable(ctx context.Context) map[string]bool {
	out := map[string]bool{claudecode.UserClaudeMD(s.configDir): true}
	for _, items := range s.projectItems(ctx) {
		for _, it := range items {
			if it.Path == "" || it.Scope == "claude.ai" || strings.HasPrefix(it.Scope, "plugin ") {
				continue
			}
			whole := it.Load == claudecode.Always && it.Kind != "skill" && it.Kind != "agent" && it.Kind != "command"
			out[it.Path] = out[it.Path] || whole
		}
	}
	return out
}

// ReadFile opens an editable file; path may start with ~.
func (s *Service) ReadFile(ctx context.Context, path string) (File, error) {
	abs := filepath.Clean(s.expandHome(path))
	always, ok := s.editable(ctx)[abs]
	if !ok {
		return File{}, problem("file-not-editable", "path", s.shortPath(abs))
	}
	b, err := os.ReadFile(abs)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return File{}, problem("read-failed", "path", s.shortPath(abs), "error", err.Error())
	}
	if len(b) > maxEditBytes {
		return File{}, problem("file-too-big", "path", s.shortPath(abs))
	}
	return File{Path: s.shortPath(abs), Name: filepath.Base(abs), Content: string(b), Version: version(b),
		Tokens: analytics.EstimateTokens(string(b)), Always: always, Exists: err == nil}, nil
}

// SaveFile writes content to an editable file through the journal, so it can be undone. It is
// refused when the file changed since the editor read it (base is the version it read).
func (s *Service) SaveFile(ctx context.Context, path, content, base string) (Result, error) {
	abs := filepath.Clean(s.expandHome(path))
	if _, ok := s.editable(ctx)[abs]; !ok {
		return Result{}, problem("file-not-editable", "path", s.shortPath(abs))
	}
	if len(content) > maxEditBytes {
		return Result{}, problem("file-too-big", "path", s.shortPath(abs))
	}
	current, err := os.ReadFile(abs)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return Result{}, problem("read-failed", "path", s.shortPath(abs), "error", err.Error())
	}
	if version(current) != base {
		return Result{}, problem("file-changed", "path", s.shortPath(abs))
	}
	if string(current) == content {
		return ok("unchanged"), nil
	}
	batch, err := fsx.NewBatch()
	if err != nil {
		return Result{}, err
	}
	short := s.shortPath(abs)
	c, err := s.journal.Write(abs, []byte(content), fsx.Meta{Rule: "edit", Title: "Edited " + short,
		Params: map[string]string{"path": short}, Batch: batch})
	if err != nil {
		return Result{}, problem("write-failed", "path", short, "error", err.Error())
	}
	return ok("saved", "change", c.ID, "version", version([]byte(content))), nil
}

// Tokens estimates the tokens of text, the same way quipu weighs files everywhere else.
func (s *Service) Tokens(text string) int64 {
	return analytics.EstimateTokens(text)
}

func version(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:8])
}
