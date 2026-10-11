package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/axvilab/quipu/internal/claudecode"
	"github.com/axvilab/quipu/internal/fsx"
	"github.com/axvilab/quipu/internal/profile"
	"github.com/axvilab/quipu/internal/scaffold"
)

// listFlag collects a flag given several times.
type listFlag []string

func (l *listFlag) String() string     { return strings.Join(*l, ",") }
func (l *listFlag) Set(v string) error { *l = append(*l, v); return nil }

func runNew(args []string) error {
	fs := flag.NewFlagSet("new", flag.ExitOnError)
	summary := fs.String("summary", "", "one or two sentences on what the project is for")
	prof := fs.String("profile", "", "Claude profile for this folder (see quipu profile)")
	dry := fs.Bool("dry-run", false, "list the files without writing them")
	var from listFlag
	fs.Var(&from, "from", "a skill folder, rule or agent of another project to copy (repeatable); "+
		"to share one between projects instead, move it to your own settings with quipu advise or the UI")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: quipu new <folder> [flags]   (the folder may be new or a fresh clone; existing files are kept)")
		fs.PrintDefaults()
	}
	// The folder may come before or after the flags.
	var folder string
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		folder, args = args[0], args[1:]
	}
	fs.Parse(args)
	if folder == "" && fs.NArg() == 1 {
		folder = fs.Arg(0)
	}
	if folder == "" {
		fs.Usage()
		os.Exit(2)
	}
	dir, err := filepath.Abs(folder)
	if err != nil {
		return err
	}

	opts := scaffold.Options{Dir: dir, Summary: *summary}
	for _, p := range from {
		kind, file, ok := claudecode.ExtensionAt(p)
		if !ok {
			return fmt.Errorf("%s is not a skill, rule or agent", p)
		}
		opts.Extensions = append(opts.Extensions, scaffold.Extension{Kind: kind, Path: file})
	}
	files, err := scaffold.Plan(opts)
	if err != nil {
		return err
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	var profiles *profile.Config
	if *prof != "" {
		if profiles, err = profile.Load(home, profilesFile(home)); err != nil {
			return err
		}
		if profiles.Find(*prof) == nil {
			return fmt.Errorf("no profile %q (see quipu profile list)", *prof)
		}
	}

	written := 0
	for _, f := range files {
		if !f.Exists {
			written++
		}
	}
	if *dry {
		fmt.Printf("quipu new would set up %s:\n", shortPath(home, dir, dir))
		for _, f := range files {
			if f.Exists {
				fmt.Printf("  %s (already there, kept)\n", f.Rel)
			} else {
				fmt.Printf("  %s\n", f.Rel)
			}
		}
		return nil
	}

	data, err := dataDir(home)
	if err != nil {
		return err
	}
	j, err := fsx.OpenJournal(filepath.Join(data, "backups"))
	if err != nil {
		return err
	}
	name := filepath.Base(dir)
	batch, err := fsx.NewBatch()
	if err != nil {
		return err
	}
	meta := fsx.Meta{Rule: "new-project", Title: "new project " + name, Params: map[string]string{"name": name}, Batch: batch}
	if _, err := scaffold.Apply(dir, files, j, meta); err != nil {
		return err
	}
	fmt.Printf("Set up %s: %d files written, %d already there and kept.\n", shortPath(home, dir, dir), written, len(files)-written)

	if profiles != nil {
		if err := profiles.SetRule(dir, *prof); err != nil {
			return err
		}
		if err := profiles.Save(); err != nil {
			return err
		}
		fmt.Printf("Claude there runs as %s.\n", *prof)
	}
	fmt.Printf("Next: cd %s && claude; once there is code, /init adds its commands to CLAUDE.md.\n", shortPath(home, dir, dir))
	return nil
}
