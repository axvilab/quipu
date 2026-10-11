package ui

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Opening the sign-in page in a private window keeps a browser that is already signed in to
// claude.ai with another account from simply confirming that one. Pages cannot open private
// windows, so quipu starts the browser itself, which works when it runs in the desktop session.
// When nothing here fits, the page still offers "Open here" and "Copy link".

// families map words found in a browser's command or desktop file to its private-window flag.
// Order matters: Edge and Opera are Chromium-based but use their own flag.
var families = []struct{ word, flag string }{
	{"edge", "--inprivate"},
	{"opera", "--private"},
	{"chrom", "--incognito"}, // chrome, chromium
	{"brave", "--incognito"},
	{"vivaldi", "--incognito"},
	{"yandex", "--incognito"},
	{"firefox", "--private-window"},
	{"librewolf", "--private-window"},
	{"waterfox", "--private-window"},
	{"floorp", "--private-window"},
	{"zen", "--private-window"},
	{"mullvad", "--private-window"},
}

// privateFlag returns the private-window flag for a browser command or desktop file name.
func privateFlag(s string) string {
	s = strings.ToLower(s)
	for _, f := range families {
		if strings.Contains(s, f.word) {
			return f.flag
		}
	}
	return ""
}

// knownCommands are browsers looked up on PATH, in the order they are offered.
var knownCommands = []struct{ cmd, name string }{
	{"google-chrome", "Chrome"}, {"google-chrome-stable", "Chrome"},
	{"chromium", "Chromium"}, {"chromium-browser", "Chromium"},
	{"brave-browser", "Brave"}, {"vivaldi", "Vivaldi"}, {"vivaldi-stable", "Vivaldi"},
	{"microsoft-edge", "Edge"}, {"opera", "Opera"},
	{"firefox", "Firefox"}, {"librewolf", "LibreWolf"}, {"zen-browser", "Zen"},
	{"floorp", "Floorp"}, {"waterfox", "Waterfox"},
}

// macApps are browsers looked up in /Applications on macOS.
var macApps = []string{
	"Google Chrome", "Chromium", "Brave Browser", "Vivaldi", "Microsoft Edge", "Opera",
	"Firefox", "LibreWolf", "Zen", "Floorp", "Waterfox",
}

// browser is one installed browser that can open a private window.
type browser struct {
	ID      string   // form value
	Name    string   // shown to the user
	Args    []string // command and private flag; the URL is appended
	Default bool
}

// installedBrowsers lists browsers that can open a private window, the default one first.
func installedBrowsers() []browser {
	if runtime.GOOS == "darwin" {
		var out []browser
		for _, app := range macApps {
			if _, err := os.Stat("/Applications/" + app + ".app"); err == nil {
				out = append(out, browser{ID: app, Name: app, Args: []string{"open", "-na", app, "--args", privateFlag(app)}})
			}
		}
		return out
	}

	var out []browser
	seen := map[string]bool{}
	// The default browser from its desktop file: this also covers Flatpak and Snap installs and
	// browsers missing from knownCommands, as long as they belong to a known family.
	if b, ok := defaultBrowserEntry(); ok {
		out = append(out, b)
		seen[b.Name] = true
	}
	for _, k := range knownCommands {
		if seen[k.name] {
			continue
		}
		if _, err := exec.LookPath(k.cmd); err == nil {
			out = append(out, browser{ID: k.cmd, Name: k.name, Args: []string{k.cmd, privateFlag(k.cmd)}})
			seen[k.name] = true
		}
	}
	return out
}

// defaultBrowserEntry reads the default browser's desktop file (xdg-settings) and turns its Exec
// line into a private-window command.
func defaultBrowserEntry() (browser, bool) {
	out, err := exec.Command("xdg-settings", "get", "default-web-browser").Output()
	if err != nil {
		return browser{}, false
	}
	file := strings.TrimSpace(string(out))
	path := findDesktopFile(file)
	if path == "" {
		return browser{}, false
	}
	name, execLine := readDesktopEntry(path)
	flag := privateFlag(file + " " + execLine)
	args := execArgs(execLine)
	if flag == "" || len(args) == 0 {
		return browser{}, false
	}
	if name == "" {
		name = strings.TrimSuffix(file, ".desktop")
	}
	return browser{ID: "default", Name: name, Args: append(args, flag), Default: true}, true
}

// findDesktopFile looks for a .desktop file in the XDG data dirs, including Flatpak and Snap exports.
func findDesktopFile(file string) string {
	home, _ := os.UserHomeDir()
	dirs := []string{os.Getenv("XDG_DATA_HOME"), filepath.Join(home, ".local", "share")}
	dirs = append(dirs, filepath.SplitList(os.Getenv("XDG_DATA_DIRS"))...)
	dirs = append(dirs, "/usr/local/share", "/usr/share", "/var/lib/flatpak/exports/share",
		filepath.Join(home, ".local", "share", "flatpak", "exports", "share"), "/var/lib/snapd/desktop")
	for _, d := range dirs {
		if d == "" {
			continue
		}
		p := filepath.Join(d, "applications", file)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// readDesktopEntry returns Name and Exec from the [Desktop Entry] group of a desktop file.
func readDesktopEntry(path string) (name, execLine string) {
	f, err := os.Open(path)
	if err != nil {
		return "", ""
	}
	defer f.Close()
	inEntry := false
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "[") {
			inEntry = line == "[Desktop Entry]"
			continue
		}
		if !inEntry {
			continue
		}
		if v, ok := strings.CutPrefix(line, "Name="); ok && name == "" {
			name = v
		}
		if v, ok := strings.CutPrefix(line, "Exec="); ok && execLine == "" {
			execLine = v
		}
	}
	return name, execLine
}

// execArgs splits an Exec line and drops field codes such as %u. Quoted arguments with spaces
// are rare in browser entries and not supported.
func execArgs(execLine string) []string {
	var args []string
	for _, f := range strings.Fields(execLine) {
		if len(f) == 2 && f[0] == '%' {
			continue
		}
		args = append(args, strings.Trim(f, `"`))
	}
	return args
}

// openPrivateWith opens url privately in the chosen browser (an ID from installedBrowsers), or
// in the first one, the default, when choice is empty.
func openPrivateWith(choice, url string) bool {
	for _, b := range installedBrowsers() {
		if choice != "" && b.ID != choice {
			continue
		}
		args := append(append([]string{}, b.Args...), url)
		return exec.Command(args[0], args[1:]...).Start() == nil
	}
	return false
}

// openPrivate opens url in a private window of the default browser, or the first one known.
func openPrivate(url string) bool {
	return openPrivateWith("", url)
}
