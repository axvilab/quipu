package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"golang.org/x/term"

	"github.com/axvilab/quipu/internal/claudecode"
	"github.com/axvilab/quipu/internal/fsx"
	"github.com/axvilab/quipu/internal/profile"
)

const profileUsage = `usage: quipu profile <command>

  list                          profiles, their login and folder rules
  add <name> [--share items]    create ~/.claude-<name>; shares skills, agents, rules, commands
                                and CLAUDE.md from ~/.claude unless --share says otherwise
                                (--share none, or a comma list)
  login <name>                  sign in to the profile's account (runs claude auth login)
  remove <name>                 forget a profile; its directory and login are kept
  rule <folder> <name>          folders below <folder> use profile <name>
  unrule <folder>               delete the rule for <folder>
  shim on|off|status [--rc]     make plain "claude" pick the profile by folder; --rc also adds or
                                removes the PATH line in your shell's startup file
  memory share|unshare|status <folder>
                                share the project's memory with the profile of its folder rule
`

func loadProfiles() (string, *profile.Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", nil, err
	}
	c, err := profile.Load(home, profilesFile(home))
	return home, c, err
}

func realClaude() (string, error) {
	self, _ := os.Executable()
	return claudecode.FindBinary(self)
}

func runProfile(ctx context.Context, args []string) error {
	if len(args) == 0 {
		fmt.Print(profileUsage)
		return nil
	}
	home, c, err := loadProfiles()
	if err != nil {
		return err
	}
	switch cmd, rest := args[0], args[1:]; cmd {
	case "list", "ls":
		return listProfiles(ctx, home, c)
	case "add":
		fs := flag.NewFlagSet("profile add", flag.ExitOnError)
		share := fs.String("share", strings.Join(profile.Shareable, ","), "items of ~/.claude to share, or none")
		name, flags := splitName(rest)
		fs.Parse(flags)
		if name == "" {
			return fmt.Errorf("usage: quipu profile add <name> [--share items]")
		}
		var items []string
		if *share != "none" && *share != "" {
			items = strings.Split(*share, ",")
		}
		p, err := c.Add(name, items)
		if err != nil {
			return err
		}
		if err := c.Save(); err != nil {
			return err
		}
		fmt.Printf("Profile %s created in %s", p.Name, shortPath(home, p.Dir, p.Dir))
		if len(p.Links) > 0 {
			var names []string
			for _, l := range p.Links {
				names = append(names, filepath.Base(l))
			}
			fmt.Printf(" (shares %s with ~/.claude)", strings.Join(names, ", "))
		}
		fmt.Printf(".\nNext: quipu profile login %s\n", p.Name)
		return nil
	case "login":
		if len(rest) != 1 || c.Find(rest[0]) == nil {
			return fmt.Errorf("usage: quipu profile login <existing profile>")
		}
		binary, err := realClaude()
		if err != nil {
			return err
		}
		code, err := profile.Run(binary, []string{"auth", "login"}, claudecode.Env(home, c.Find(rest[0]).Dir))
		if err == nil && code != 0 {
			err = fmt.Errorf("claude auth login exited with %d", code)
		}
		return err
	case "remove", "rm":
		if len(rest) != 1 {
			return fmt.Errorf("usage: quipu profile remove <name>")
		}
		dir, err := c.Remove(rest[0])
		if err != nil {
			return err
		}
		if err := c.Save(); err != nil {
			return err
		}
		fmt.Printf("Profile %s removed. Its directory with the login and sessions is kept: %s\n", rest[0], dir)
		return nil
	case "shim":
		return runShim(home, rest)
	case "memory":
		return runMemory(home, c, rest)
	case "rule":
		if len(rest) != 2 {
			return fmt.Errorf("usage: quipu profile rule <folder> <name>")
		}
		folder, err := filepath.Abs(rest[0])
		if err != nil {
			return err
		}
		if err := c.SetRule(folder, rest[1]); err != nil {
			return err
		}
		if err := c.Save(); err != nil {
			return err
		}
		fmt.Printf("Claude in %s and below runs as %s (through quipu run).\n", shortPath(home, folder, folder), rest[1])
		return nil
	case "unrule":
		if len(rest) != 1 {
			return fmt.Errorf("usage: quipu profile unrule <folder>")
		}
		folder, err := filepath.Abs(rest[0])
		if err != nil {
			return err
		}
		if !c.DeleteRule(folder) {
			return fmt.Errorf("no rule for %s", folder)
		}
		return c.Save()
	}
	fmt.Print(profileUsage)
	return fmt.Errorf("unknown profile command %q", args[0])
}

func listProfiles(ctx context.Context, home string, c *profile.Config) error {
	binary, berr := realClaude()
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "PROFILE\tDIR\tACCOUNT\tSHARES")
	for _, p := range c.Profiles {
		account := "?"
		if berr == nil {
			actx, cancel := context.WithTimeout(ctx, 15*time.Second)
			a, err := claudecode.AuthStatus(actx, binary, home, p.Dir)
			cancel()
			switch {
			case err != nil:
				account = "unknown"
			case !a.LoggedIn:
				account = "not logged in"
			default:
				account = strings.TrimSpace(a.Email + " " + a.SubscriptionType)
			}
		}
		var shares []string
		for _, l := range p.Links {
			shares = append(shares, filepath.Base(l))
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", p.Name, shortPath(home, p.Dir, p.Dir), account, strings.Join(shares, ","))
	}
	w.Flush()
	if len(c.Rules) > 0 {
		fmt.Println("\nFolder rules (quipu run):")
		for _, r := range c.Rules {
			fmt.Printf("  %s → %s\n", shortPath(home, r.Path, r.Path), r.Profile)
		}
	}
	return nil
}

// runClaude is `quipu run`: pick a profile and start the real claude under it.
func runClaude(args []string) (int, error) {
	explicit := ""
	if len(args) >= 2 && (args[0] == "--profile" || args[0] == "-P") {
		explicit, args = args[1], args[2:]
	} else if len(args) >= 1 && strings.HasPrefix(args[0], "--profile=") {
		explicit, args = strings.TrimPrefix(args[0], "--profile="), args[1:]
	}
	if len(args) > 0 && args[0] == "--" {
		args = args[1:]
	}
	return runClaudeWith(args, explicit)
}

// runClaudeWith runs the real claude with args under the explicit profile, or the one the folder
// rules, a question or the default give.
func runClaudeWith(args []string, explicit string) (int, error) {

	home, c, err := loadProfiles()
	if err != nil {
		return 1, err
	}
	binary, err := realClaude()
	if err != nil {
		return 1, err
	}
	cwd, _ := os.Getwd()

	name := explicit
	interactive := profile.Interactive(args) && isTerminal(os.Stdin)
	if name == "" {
		if r, ok := c.RuleFor(cwd); ok {
			name = r.Profile
			// One line, so a rule remembered by mistake is noticed at once.
			if interactive && isTerminal(os.Stderr) {
				fmt.Fprintf(os.Stderr, "quipu: %s (rule for %s)\n", r.Profile, shortPath(home, r.Path, r.Path))
			}
		}
	}
	if name == "" && len(c.Profiles) > 1 && interactive {
		in := bufio.NewReader(os.Stdin)
		name = askProfile(c, in, os.Stderr)
		if folder := profile.RememberFolder(cwd, home); folder != "" && askRemember(name, shortPath(home, folder, folder), in, os.Stderr) {
			if err := rememberRule(home, folder, name); err != nil {
				fmt.Fprintln(os.Stderr, "quipu: not remembered:", err)
			} else {
				offerSharedMemory(home, folder, name, in)
			}
		}
	}
	if name == "" {
		name = profile.Default
	}
	p := c.Find(name)
	if p == nil {
		return 1, fmt.Errorf("no profile %q (see quipu profile list)", name)
	}
	return profile.Run(binary, args, claudecode.Env(home, p.Dir))
}

func askProfile(c *profile.Config, in *bufio.Reader, out io.Writer) string {
	fmt.Fprintln(out, "Which Claude account for this folder?")
	for i, p := range c.Profiles {
		fmt.Fprintf(out, "  %d) %s\n", i+1, p.Name)
	}
	fmt.Fprintf(out, "Choose [1]: ")
	line, _ := in.ReadString('\n')
	if n, err := strconv.Atoi(strings.TrimSpace(line)); err == nil && n >= 1 && n <= len(c.Profiles) {
		return c.Profiles[n-1].Name
	}
	return c.Profiles[0].Name
}

// askRemember asks whether to keep the choice as a folder rule; Enter means yes.
func askRemember(name, folder string, in *bufio.Reader, out io.Writer) bool {
	fmt.Fprintf(out, "Remember %s for %s? [Y/n]: ", name, folder)
	line, _ := in.ReadString('\n')
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "", "y", "yes", "т", "так", "д", "да":
		return true
	}
	return false
}

// offerSharedMemory asks to share the project's memory when another profile has some, so the
// project keeps what Claude learned about it under that account.
func offerSharedMemory(home, folder, name string, in *bufio.Reader) {
	c, err := profile.Load(home, profilesFile(home))
	if err != nil {
		return
	}
	m := profile.ProjectMemory(c, home, folder, name)
	if m.Shared || m.SourceFiles == 0 {
		return
	}
	fmt.Fprintf(os.Stderr, "%s has memory of this project (%d files). Share it with %s? [Y/n]: ", m.Source, m.SourceFiles, name)
	line, _ := in.ReadString('\n')
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "", "y", "yes", "т", "так", "д", "да":
	default:
		return
	}
	journal, err := openJournal(home)
	if err == nil {
		_, err = profile.ShareMemory(c, home, folder, name, journal)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "quipu: memory not shared:", err)
		return
	}
	fmt.Fprintf(os.Stderr, "quipu: %s now uses the same memory as %s here\n", name, m.Source)
}

// rememberRule saves a folder rule. It loads the profiles again first, so a rule another terminal
// saved meanwhile is kept.
func rememberRule(home, folder, name string) error {
	c, err := profile.Load(home, profilesFile(home))
	if err != nil {
		return err
	}
	if err := c.SetRule(folder, name); err != nil {
		return err
	}
	if err := c.Save(); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "quipu: remembered; change it on the Profiles page or with quipu profile unrule %s\n", shortPath(home, folder, folder))
	return nil
}

// isTerminal is true only for a real terminal; /dev/null and pipes do not count.
func isTerminal(f *os.File) bool {
	return term.IsTerminal(int(f.Fd()))
}

// splitName takes the first non-flag argument as the name and returns the rest as flags, so both
// "add work --share none" and "add --share none work" work.
func splitName(args []string) (string, []string) {
	for i, a := range args {
		if !strings.HasPrefix(a, "-") && (i == 0 || !strings.HasPrefix(args[i-1], "-") || strings.Contains(args[i-1], "=")) {
			return a, append(append([]string{}, args[:i]...), args[i+1:]...)
		}
	}
	return "", args
}

// runMemory shares a project's memory with the profile its folder rule names, or stops it.
func runMemory(home string, c *profile.Config, args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("usage: quipu profile memory share|unshare|status <folder>")
	}
	folder, err := filepath.Abs(args[1])
	if err != nil {
		return err
	}
	r, ok := c.RuleFor(folder)
	if !ok {
		return fmt.Errorf("no folder rule covers %s; add one with quipu profile rule", shortPath(home, folder, folder))
	}
	project := r.Path
	journal, err := openJournal(home)
	if err != nil {
		return err
	}
	switch args[0] {
	case "status":
		m := profile.ProjectMemory(c, home, project, r.Profile)
		switch {
		case m.Shared:
			fmt.Printf("%s uses the memory of %s for %s.\n", r.Profile, m.From, shortPath(home, project, project))
		case m.SourceFiles > 0:
			fmt.Printf("%s has its own memory (%d files); %s has %d. Share with: quipu profile memory share %s\n",
				r.Profile, m.Own, m.Source, m.SourceFiles, shortPath(home, project, project))
		default:
			fmt.Printf("%s has its own memory (%d files); no other profile has any.\n", r.Profile, m.Own)
		}
		return nil
	case "share":
		if _, err := profile.ShareMemory(c, home, project, r.Profile, journal); err != nil {
			return err
		}
		fmt.Printf("%s now shares the memory of %s. Undo it under Advice, in Changes made by quipu (quipu ui).\n",
			r.Profile, shortPath(home, project, project))
		return nil
	case "unshare":
		if _, err := profile.UnshareMemory(c, home, project, r.Profile, journal); err != nil {
			return err
		}
		fmt.Printf("%s has its own memory of %s again, starting from a copy.\n", r.Profile, shortPath(home, project, project))
		return nil
	}
	return fmt.Errorf("usage: quipu profile memory share|unshare|status <folder>")
}

// openJournal opens quipu's change journal, so CLI edits can be undone in the UI.
func openJournal(home string) (*fsx.Journal, error) {
	data, err := dataDir(home)
	if err != nil {
		return nil, err
	}
	return fsx.OpenJournal(filepath.Join(data, "backups"))
}
