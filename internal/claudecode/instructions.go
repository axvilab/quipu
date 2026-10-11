package claudecode

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Load tells when Claude Code puts an item into the context.
type Load string

const (
	Always   Load = "always"    // in every session, from the first request
	OnDemand Load = "on-demand" // only when Claude touches matching files or invokes it
)

// Item is one piece of instructions Claude Code may send for a project.
type Item struct {
	Load  Load
	Scope string // "user", "claude.ai", "project", "local", "memory" or "plugin <name>"
	Kind  string // "CLAUDE.md", "import", "rule", "skill", "agent", "command", "memory"
	Name  string
	Path  string
	Text  string // what enters the context: a whole file, or a skill's name and description
	From  string // for imports: the file that contains the @reference
	Ref   string // for imports: the reference as written, e.g. "docs/adr/README.md"
}

// Memory files are loaded only up to these limits.
const (
	memoryMaxLines = 200
	memoryMaxBytes = 25 << 10
	importMaxDepth = 5
)

// Instructions lists everything Claude Code loads for the project at projectPath when it runs
// with configDir as its config directory. Missing files are simply absent from the result.
func Instructions(home, configDir, projectPath string) []Item {
	c := collector{home: home, seen: map[string]bool{}}

	c.claudeMD(UserClaudeMD(configDir), "user")
	c.rules(filepath.Join(configDir, "rules"), "user")

	// CLAUDE.md files of the project and all its parents, outermost first.
	var dirs []string
	for dir := projectPath; ; dir = filepath.Dir(dir) {
		dirs = append([]string{dir}, dirs...)
		if dir == filepath.Dir(dir) {
			break
		}
	}
	for _, dir := range dirs {
		c.claudeMD(filepath.Join(dir, "CLAUDE.md"), "project")
		c.claudeMD(filepath.Join(dir, ".claude", "CLAUDE.md"), "project")
		c.claudeMD(filepath.Join(dir, "CLAUDE.local.md"), "local")
	}
	c.rules(filepath.Join(projectPath, ".claude", "rules"), "project")
	c.subdirClaudeMD(projectPath)

	c.memory(memoryDir(configDir, projectPath))

	c.extensions(configDir, "user")
	c.extensions(filepath.Join(projectPath, ".claude"), "project")
	for name, dir := range enabledPlugins(configDir, projectPath) {
		c.extensions(dir, "plugin "+name)
	}
	return c.items
}

// UserExtensionDir is where the profile keeps its own skills, rules or agents (kind "skill",
// "rule" or "agent"); those apply in every project.
func UserExtensionDir(configDir, kind string) string {
	return filepath.Join(configDir, kind+"s")
}

// UserClaudeMD is the profile's own CLAUDE.md, loaded in every project; it may not exist yet.
func UserClaudeMD(configDir string) string {
	return filepath.Join(configDir, "CLAUDE.md")
}

type collector struct {
	home  string
	items []Item
	seen  map[string]bool // files already added, so a file reached twice counts once
}

func (c *collector) add(it Item) {
	key := string(it.Load) + it.Path
	if c.seen[key] {
		return
	}
	c.seen[key] = true
	c.items = append(c.items, it)
}

func (c *collector) claudeMD(path, scope string) {
	text, ok := readFile(path)
	if !ok {
		return
	}
	c.add(Item{Load: Always, Scope: scope, Kind: "CLAUDE.md", Name: filepath.Base(path), Path: path, Text: text})
	c.imports(path, text, scope, Always, 1)
}

// importRe matches "@path" references. Code spans and fenced blocks are removed before matching.
var (
	importRe    = regexp.MustCompile(`(?:^|\s)@((?:~/|\.{0,2}/)?[\w./-]+)`)
	codeBlockRe = regexp.MustCompile("(?s)```.*?```|`[^`\n]*`")
)

func (c *collector) imports(from, text, scope string, load Load, depth int) {
	if depth > importMaxDepth {
		return
	}
	for _, m := range importRe.FindAllStringSubmatch(codeBlockRe.ReplaceAllString(text, ""), -1) {
		ref := m[1]
		path := filepath.Join(filepath.Dir(from), ref)
		if rest, ok := strings.CutPrefix(ref, "~/"); ok {
			path = filepath.Join(c.home, rest)
		} else if filepath.IsAbs(ref) {
			path = ref
		}
		body, ok := readFile(path)
		if !ok || c.seen[string(load)+path] {
			continue
		}
		c.add(Item{Load: load, Scope: scope, Kind: "import", Name: ref, Path: path, Text: body, From: from, Ref: ref})
		c.imports(path, body, scope, load, depth+1)
	}
}

// rules adds .claude/rules/**/*.md: rules with "paths:" load only for matching files.
func (c *collector) rules(dir, scope string) {
	for _, path := range findFiles(dir, func(p string) bool { return strings.HasSuffix(p, ".md") }) {
		text, _ := readFile(path)
		fm, body := parseFrontmatter(text)
		load := Always
		if len(fm.Paths) > 0 {
			load = OnDemand
		}
		rel, _ := filepath.Rel(dir, path)
		c.add(Item{Load: load, Scope: scope, Kind: "rule", Name: rel, Path: path, Text: body})
	}
}

// subdirClaudeMD adds CLAUDE.md files below the project root; Claude reads them when it works
// with files in that directory.
func (c *collector) subdirClaudeMD(root string) {
	for _, path := range findFiles(root, func(p string) bool { return filepath.Base(p) == "CLAUDE.md" }) {
		if filepath.Dir(path) == root || filepath.Dir(path) == filepath.Join(root, ".claude") {
			continue
		}
		text, _ := readFile(path)
		rel, _ := filepath.Rel(root, path)
		c.add(Item{Load: OnDemand, Scope: "project", Kind: "CLAUDE.md", Name: rel, Path: path, Text: text})
	}
}

func (c *collector) memory(dir string) {
	if text, ok := readFile(filepath.Join(dir, "MEMORY.md")); ok {
		c.add(Item{Load: Always, Scope: "memory", Kind: "memory", Name: "MEMORY.md",
			Path: filepath.Join(dir, "MEMORY.md"), Text: truncateMemory(text)})
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.IsDir() || e.Name() == "MEMORY.md" || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		text, _ := readFile(path)
		c.add(Item{Load: OnDemand, Scope: "memory", Kind: "memory", Name: e.Name(), Path: path, Text: text})
	}
}

// extensions adds skills, agents and commands under base: their name and description are listed
// in every session, the body is loaded when used.
func (c *collector) extensions(base, scope string) {
	skills := findFiles(filepath.Join(base, "skills"), func(p string) bool { return filepath.Base(p) == "SKILL.md" })
	for _, path := range skills {
		skillScope := scope
		// Skills enabled on claude.ai are synced into <config>/skills/synced/<id>/<name>/.
		if strings.Contains(path, string(filepath.Separator)+filepath.Join("skills", "synced")+string(filepath.Separator)) {
			skillScope = "claude.ai"
		}
		c.extension(path, skillScope, "skill", filepath.Base(filepath.Dir(path)))
	}
	isMD := func(p string) bool { return strings.HasSuffix(p, ".md") }
	for _, path := range findFiles(filepath.Join(base, "agents"), isMD) {
		c.extension(path, scope, "agent", strings.TrimSuffix(filepath.Base(path), ".md"))
	}
	for _, path := range findFiles(filepath.Join(base, "commands"), isMD) {
		c.extension(path, scope, "command", strings.TrimSuffix(filepath.Base(path), ".md"))
	}
}

func (c *collector) extension(path, scope, kind, fallbackName string) {
	text, _ := readFile(path)
	fm, body := parseFrontmatter(text)
	name := fm.Name
	if name == "" {
		name = fallbackName
	}
	c.add(Item{Load: Always, Scope: scope, Kind: kind, Name: name, Path: path, Text: name + ": " + fm.Description})
	c.add(Item{Load: OnDemand, Scope: scope, Kind: kind, Name: name, Path: path, Text: body})
}

// MemoryDir is the project's auto-memory directory, where Claude Code keeps MEMORY.md.
func MemoryDir(configDir, projectPath string) string {
	return memoryDir(configDir, projectPath)
}

// memoryDir is the project's auto-memory directory: autoMemoryDirectory from user or local
// settings (project settings may not set it), else <config>/projects/<encoded path>/memory.
func memoryDir(configDir, projectPath string) string {
	for _, f := range []string{
		filepath.Join(projectPath, ".claude", "settings.local.json"),
		filepath.Join(configDir, "settings.json"),
	} {
		var s struct {
			AutoMemoryDirectory string `json:"autoMemoryDirectory"`
		}
		if readJSON(f, &s) && s.AutoMemoryDirectory != "" {
			dir := s.AutoMemoryDirectory
			if rest, ok := strings.CutPrefix(dir, "~/"); ok {
				dir = filepath.Join(filepath.Dir(configDir), rest)
			}
			return dir
		}
	}
	return filepath.Join(configDir, "projects", ProjectDirName(projectPath), "memory")
}

// enabledPlugins maps each plugin enabled for the project to its install directory.
// Later settings files override earlier ones: user, project, local.
func enabledPlugins(configDir, projectPath string) map[string]string {
	enabled := map[string]bool{}
	for _, f := range []string{
		filepath.Join(configDir, "settings.json"),
		filepath.Join(projectPath, ".claude", "settings.json"),
		filepath.Join(projectPath, ".claude", "settings.local.json"),
	} {
		var s struct {
			EnabledPlugins map[string]json.RawMessage `json:"enabledPlugins"`
		}
		if !readJSON(f, &s) {
			continue
		}
		for id, raw := range s.EnabledPlugins {
			var on bool
			// Besides true/false, a value can be a list of version constraints, meaning enabled.
			enabled[id] = json.Unmarshal(raw, &on) != nil || on
		}
	}

	var installed struct {
		Plugins map[string][]struct {
			InstallPath string `json:"installPath"`
		} `json:"plugins"`
	}
	readJSON(filepath.Join(configDir, "plugins", "installed_plugins.json"), &installed)

	out := map[string]string{}
	for id, on := range enabled {
		if inst := installed.Plugins[id]; on && len(inst) > 0 && inst[0].InstallPath != "" {
			out[strings.SplitN(id, "@", 2)[0]] = inst[0].InstallPath
		}
	}
	return out
}

func truncateMemory(text string) string {
	lines := strings.SplitAfter(text, "\n")
	if len(lines) > memoryMaxLines {
		text = strings.Join(lines[:memoryMaxLines], "")
	}
	if len(text) > memoryMaxBytes {
		text = text[:memoryMaxBytes]
	}
	return text
}

// skipDirs are never searched for instruction files.
var skipDirs = map[string]bool{"node_modules": true, "vendor": true, ".git": true, "dist": true, "build": true, "target": true}

// findFiles walks dir (at most 6 levels deep, skipping dependency and hidden directories other
// than .claude) and returns the files accepted by keep, sorted.
func findFiles(dir string, keep func(string) bool) []string {
	var out []string
	filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			rel, _ := filepath.Rel(dir, path)
			deep := strings.Count(rel, string(filepath.Separator)) >= 6
			hidden := path != dir && strings.HasPrefix(name, ".") && name != ".claude"
			if deep || hidden || skipDirs[name] {
				return filepath.SkipDir
			}
			return nil
		}
		if keep(path) {
			out = append(out, path)
		}
		return nil
	})
	sort.Strings(out)
	return out
}

func readFile(path string) (string, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	return string(b), true
}

func readJSON(path string, v any) bool {
	b, err := os.ReadFile(path)
	return err == nil && json.Unmarshal(b, v) == nil
}
