package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/axvilab/quipu/internal/backup"
	"github.com/axvilab/quipu/internal/fsx"
)

func runBackup(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("backup", flag.ExitOnError)
	out := fs.String("out", "", "archive to write (default ~/quipu-backups/quipu-backup-<date>.tar.gz)")
	repo := fs.String("git", "", "git working copy to save into instead of an archive (a clone of your private repository)")
	push := fs.Bool("push", false, "with --git: push after committing")
	allow := fs.Bool("allow-secrets", false, "back up even if something looks like a credential")
	fs.Parse(args)

	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	var exclude map[string]bool
	if data, err := dataDir(home); err == nil {
		exclude = backup.LoadSettings(data).Excluded() // projects left out in the window stay out here too
	}
	m, entries, err := backup.Collect(home, profilesFile(home), version, exclude)
	if err != nil {
		return err
	}
	if secrets := backup.Scan(entries); len(secrets) > 0 && !*allow {
		fmt.Println("These files look like they contain credentials:")
		for _, s := range secrets {
			fmt.Printf("  %s:%d  (%s)\n", s.Path, s.Line, s.Kind)
		}
		return errors.New("nothing was written; remove the credentials (for example, move them out of settings.json env) or run again with --allow-secrets")
	}

	if *repo != "" {
		changed, err := backup.SaveToGit(ctx, *repo, m, entries, *push)
		if err != nil {
			return err
		}
		switch {
		case !changed:
			fmt.Printf("Nothing changed since the last backup in %s.\n", *repo)
		case *push:
			fmt.Printf("Committed and pushed %d files to %s.\n", len(entries), *repo)
		default:
			fmt.Printf("Committed %d files in %s (push with --push or git push).\n", len(entries), *repo)
		}
		return nil
	}

	path := *out
	if path == "" {
		path = filepath.Join(home, "quipu-backups", "quipu-backup-"+time.Now().Format("2006-01-02-1504")+".tar.gz")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if err := backup.WriteArchive(f, m, entries); err != nil {
		f.Close()
		os.Remove(path)
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	fmt.Printf("Backed up %d files (%d shared links) to %s\n", len(entries), len(m.Links), path)
	return nil
}

func runRestore(args []string) error {
	fs := flag.NewFlagSet("restore", flag.ExitOnError)
	apply := fs.Bool("apply", false, "write the files (without it, only show what would change)")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: quipu restore [--apply] <backup archive or folder>")
		fs.PrintDefaults()
	}
	fs.Parse(args)
	if fs.NArg() != 1 {
		fs.Usage()
		os.Exit(2)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	m, entries, err := readBackup(fs.Arg(0))
	if err != nil {
		return err
	}
	plan := backup.PlanRestore(m, entries, home, profilesFile(home))
	created, changed := plan.Count()
	fmt.Printf("Backup of %s from %s (quipu %s)\n\n", m.Created.Local().Format("2006-01-02 15:04"), m.Home, m.Version)
	for _, f := range plan.Files {
		if f.State != "same" {
			fmt.Printf("  %-8s %s\n", f.State, shortPath(home, f.Path, f.Path))
		}
	}
	for _, l := range plan.Links {
		if l.State != "same" {
			fmt.Printf("  %-8s %s -> %s\n", "link "+l.State, shortPath(home, l.Path, l.Path), l.Target)
		}
	}
	fmt.Printf("\n%d new, %d changed, %d already the same.\n", created, changed, len(plan.Files)-created-changed)
	if !*apply {
		if created+changed > 0 {
			fmt.Println("Run again with --apply to write them; quipu keeps a copy, so it can be undone in quipu ui.")
		}
		return nil
	}
	dir, err := dataDir(home)
	if err != nil {
		return err
	}
	j, err := fsx.OpenJournal(filepath.Join(dir, "backups"))
	if err != nil {
		return err
	}
	if _, err := plan.Apply(j, m.Version); err != nil {
		return err
	}
	fmt.Println("Restored. Undo it under Advice, in Changes made by quipu (quipu ui).")
	return nil
}

// readBackup opens an archive file or a backup folder (a git working copy).
func readBackup(path string) (backup.Manifest, []backup.Entry, error) {
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
