package ui

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/axvilab/quipu/internal/backup"
)

// BackupInfo is the backup page: where backups go and the archives made so far.
type BackupInfo struct {
	Folder   string    `json:"folder"`   // where archives are written, with ~
	GitDir   string    `json:"gitDir"`   // the repository working copy chosen last time, with ~
	Archives []Archive `json:"archives"` // newest first
}

// Archive is a backup file in the backup folder.
type Archive struct {
	Path string `json:"path"`
	Name string `json:"name"`
	Size int64  `json:"size"`
	Time string `json:"time"`
}

// BackupResult answers a backup: where it went, or the credentials that stopped it.
type BackupResult struct {
	OK      bool            `json:"ok"`
	Path    string          `json:"path,omitempty"`
	Files   int             `json:"files"`
	Changed bool            `json:"changed"`
	Secrets []backup.Secret `json:"secrets"`
}

// RestorePlan is what restoring a backup would do, for the preview.
type RestorePlan struct {
	Source  string        `json:"source"`
	Created string        `json:"created"`
	From    string        `json:"from"` // home of the machine it came from
	Version string        `json:"version"`
	New     int           `json:"new"`
	Changed int           `json:"changed"`
	Same    int           `json:"same"`
	Files   []RestoreFile `json:"files"` // new and changed only
	Links   int           `json:"links"` // symlinks to create
}

type RestoreFile struct {
	Path  string `json:"path"`
	State string `json:"state"`
}

func (s *Service) backupFolder() string { return filepath.Join(s.home, "quipu-backups") }

type backupSettings struct {
	GitDir string `json:"gitDir"`
}

func (s *Service) backupSettings() backupSettings {
	var b backupSettings
	if data, err := os.ReadFile(filepath.Join(s.dataDir, "backup.json")); err == nil {
		json.Unmarshal(data, &b)
	}
	return b
}

// BackupInfo lists the archives in the backup folder.
func (s *Service) BackupInfo() (BackupInfo, error) {
	out := BackupInfo{Folder: s.shortPath(s.backupFolder()), GitDir: s.shortPath(s.backupSettings().GitDir), Archives: []Archive{}}
	if out.GitDir == "." {
		out.GitDir = ""
	}
	entries, _ := os.ReadDir(s.backupFolder())
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".tar.gz") {
			continue
		}
		fi, err := e.Info()
		if err != nil {
			continue
		}
		out.Archives = append(out.Archives, Archive{Path: s.shortPath(filepath.Join(s.backupFolder(), e.Name())), Name: e.Name(),
			Size: fi.Size(), Time: fi.ModTime().Format(time.RFC3339)})
	}
	sort.Slice(out.Archives, func(i, j int) bool { return out.Archives[i].Time > out.Archives[j].Time })
	return out, nil
}

// CreateBackup writes an archive to the backup folder, or, with gitDir, saves into that working
// copy (pushing when asked). Credentials stop it unless allowSecrets.
func (s *Service) CreateBackup(ctx context.Context, gitDir string, push, allowSecrets bool) (BackupResult, error) {
	m, entries, err := backup.Collect(s.home, s.ProfilesFile, s.version)
	if err != nil {
		return BackupResult{}, problem("backup-failed", "error", err.Error())
	}
	out := BackupResult{Files: len(entries), Secrets: backup.Scan(entries)}
	if len(out.Secrets) > 0 && !allowSecrets {
		return out, nil
	}
	out.Secrets = []backup.Secret{}
	if gitDir != "" {
		dir := filepath.Clean(s.expandHome(gitDir))
		b, _ := json.Marshal(backupSettings{GitDir: dir})
		os.WriteFile(filepath.Join(s.dataDir, "backup.json"), b, 0o600)
		changed, err := backup.SaveToGit(ctx, dir, m, entries, push)
		if err != nil {
			return BackupResult{}, problem("backup-failed", "error", err.Error())
		}
		out.OK, out.Path, out.Changed = true, s.shortPath(dir), changed
		return out, nil
	}
	if err := os.MkdirAll(s.backupFolder(), 0o700); err != nil {
		return BackupResult{}, problem("backup-failed", "error", err.Error())
	}
	path := filepath.Join(s.backupFolder(), "quipu-backup-"+time.Now().Format("2006-01-02-150405")+".tar.gz")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return BackupResult{}, problem("backup-failed", "error", err.Error())
	}
	if err := backup.WriteArchive(f, m, entries); err != nil {
		f.Close()
		os.Remove(path)
		return BackupResult{}, problem("backup-failed", "error", err.Error())
	}
	if err := f.Close(); err != nil {
		return BackupResult{}, problem("backup-failed", "error", err.Error())
	}
	out.OK, out.Path, out.Changed = true, s.shortPath(path), true
	return out, nil
}

// readBackup opens an archive or a backup folder (a repository working copy).
func (s *Service) readBackup(source string) (backup.Manifest, []backup.Entry, error) {
	path := filepath.Clean(s.expandHome(source))
	if fi, err := os.Stat(path); err == nil && fi.IsDir() {
		return backup.ReadDir(path)
	}
	f, err := os.Open(path)
	if err != nil {
		return backup.Manifest{}, nil, err
	}
	defer f.Close()
	return backup.ReadArchive(f)
}

// PlanRestore shows what restoring source would change; nothing is written.
func (s *Service) PlanRestore(source string) (RestorePlan, error) {
	m, entries, err := s.readBackup(source)
	if err != nil {
		return RestorePlan{}, problem("backup-unreadable", "error", err.Error())
	}
	plan := backup.PlanRestore(m, entries, s.home, s.ProfilesFile)
	created, changed := plan.Count()
	out := RestorePlan{Source: s.shortPath(filepath.Clean(s.expandHome(source))), Created: m.Created.Format(time.RFC3339),
		From: m.Home, Version: m.Version, New: created, Changed: changed, Same: len(plan.Files) - created - changed, Files: []RestoreFile{}}
	for _, f := range plan.Files {
		if f.State != "same" {
			out.Files = append(out.Files, RestoreFile{Path: s.shortPath(f.Path), State: f.State})
		}
	}
	for _, l := range plan.Links {
		if l.State == "new" {
			out.Links++
		}
	}
	return out, nil
}

// ApplyRestore writes the backup through the journal as one batch, undone at once.
func (s *Service) ApplyRestore(source string) (Result, error) {
	m, entries, err := s.readBackup(source)
	if err != nil {
		return Result{}, problem("backup-unreadable", "error", err.Error())
	}
	plan := backup.PlanRestore(m, entries, s.home, s.ProfilesFile)
	id, err := plan.Apply(s.journal, m.Version)
	if err != nil {
		return Result{}, problem("restore-failed", "error", err.Error())
	}
	if id == "" {
		return ok("unchanged"), nil
	}
	return ok("restored", "change", id), nil
}
