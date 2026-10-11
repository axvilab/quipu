package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"golang.org/x/term"

	"github.com/axvilab/quipu/internal/claudecode"
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
	if name == "" {
		name = c.ForFolder(cwd)
	}
	if name == "" && len(c.Profiles) > 1 && profile.Interactive(args) && isTerminal(os.Stdin) {
		name = askProfile(c)
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

func askProfile(c *profile.Config) string {
	fmt.Fprintln(os.Stderr, "Which Claude account for this folder?")
	for i, p := range c.Profiles {
		fmt.Fprintf(os.Stderr, "  %d) %s\n", i+1, p.Name)
	}
	fmt.Fprintf(os.Stderr, "Choose [1]: ")
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	if n, err := strconv.Atoi(strings.TrimSpace(line)); err == nil && n >= 1 && n <= len(c.Profiles) {
		return c.Profiles[n-1].Name
	}
	return c.Profiles[0].Name
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
