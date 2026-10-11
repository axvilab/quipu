package advise

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/axvilab/quipu/internal/analytics"
	"github.com/axvilab/quipu/internal/claudecode"
)

const (
	minLine         = 20 // shorter lines (and headings, fences) never count as a duplicate alone
	minSingleBlock  = 60 // a one-line block shared between projects must be at least this long
	minSharedCopies = 3  // a block is worth moving when this many projects carry it
)

// norm is the comparable form of a line, or "" when the line does not count on its own.
func norm(line string) string {
	s := strings.Join(strings.Fields(line), " ")
	if len(s) < minLine || strings.HasPrefix(s, "#") || strings.HasPrefix(s, "```") {
		return ""
	}
	return s
}

// run is a stretch of consecutive counting lines in a file.
type run struct {
	raw   string   // the lines as written, with their line breaks
	norms []string // the same lines normalized
}

// runs splits text into maximal stretches of consecutive lines for which keep is true.
func runs(text string, keep func(norm string) bool) []run {
	var out []run
	var cur run
	flush := func() {
		if len(cur.norms) > 0 {
			out = append(out, cur)
		}
		cur = run{}
	}
	for _, line := range strings.SplitAfter(text, "\n") {
		n := norm(line)
		if n == "" || !keep(n) {
			flush()
			continue
		}
		cur.raw += line
		cur.norms = append(cur.norms, n)
	}
	flush()
	return out
}

// findRun returns the raw text of the stretch of lines in text that normalizes to norms.
func findRun(text string, norms []string) (string, bool) {
	lines := strings.SplitAfter(text, "\n")
	for i := 0; i+len(norms) <= len(lines); i++ {
		match := true
		for j, n := range norms {
			if norm(lines[i+j]) != n {
				match = false
				break
			}
		}
		if match {
			return strings.Join(lines[i:i+len(norms)], ""), true
		}
	}
	return "", false
}

// withHeading extends raw (which occurs once in text) up to the section heading above it when
// raw is all that section holds, so moving the block leaves no empty heading behind. Only
// subheadings (## and deeper) move; the file's title stays.
func withHeading(text, raw string) string {
	i := strings.Index(text, raw)
	rest := strings.TrimLeft(text[i+len(raw):], "\n")
	if rest != "" && !strings.HasPrefix(rest, "#") {
		return raw // more text follows in the same section
	}
	before := strings.TrimRight(text[:i], "\n")
	start := strings.LastIndex(before, "\n") + 1
	if !strings.HasPrefix(before[start:], "##") {
		return raw
	}
	return text[start : i+len(raw)]
}

// withBlankAfter extends raw (which occurs once in text) by one following blank line when a
// blank line also precedes it, so removing it does not leave two blank lines in a row.
func withBlankAfter(text, raw string) string {
	i := strings.Index(text, raw)
	before, after := text[:i], text[i+len(raw):]
	if (before == "" || strings.HasSuffix(before, "\n\n")) && strings.HasPrefix(after, "\n") {
		return raw + "\n"
	}
	return raw
}

// lineOf is the 1-based line number at which raw starts in text.
func lineOf(text, raw string) int {
	return strings.Count(text[:strings.Index(text, raw)], "\n") + 1
}

// hunk starts the changes of path at line in a Diff.
func hunk(path string, line int) string {
	return fmt.Sprintf("@@ %s:%d\n", path, line)
}

// appendLine is the line at which an Append edit's text starts in the file.
func appendLine(path string) int {
	b, _ := os.ReadFile(path)
	s := strings.TrimRight(string(b), "\n")
	if s == "" {
		return 1
	}
	return strings.Count(s, "\n") + 3 // after the last line and one blank line
}

// diffLines renders text as removed (sign "-") or added ("+") lines.
func diffLines(sign, text string) string {
	var b strings.Builder
	for _, line := range strings.Split(strings.TrimRight(text, "\n"), "\n") {
		b.WriteString(sign + " " + line + "\n")
	}
	return b.String()
}

// userLines are the normalized lines every session of the profile already gets from the user's
// own always-loaded files.
func userLines(items []claudecode.Item) map[string]bool {
	seen := map[string]bool{}
	for _, it := range items {
		if it.Scope != "user" || it.Load != claudecode.Always {
			continue
		}
		if it.Kind != "CLAUDE.md" && it.Kind != "rule" && it.Kind != "import" {
			continue
		}
		for _, line := range strings.Split(it.Text, "\n") {
			if n := norm(line); n != "" {
				seen[n] = true
			}
		}
	}
	return seen
}

// repeatedInSession flags lines of a project's always-loaded CLAUDE.md or rule that the user's
// own files already send in the same session, and removes the project copy.
func repeatedInSession(in Input) []Finding {
	seen := userLines(in.Items)
	if len(seen) == 0 {
		return nil
	}
	var out []Finding
	for _, it := range in.Items {
		if it.Load != claudecode.Always || (it.Scope != "project" && it.Scope != "local") {
			continue
		}
		if it.Kind != "CLAUDE.md" && it.Kind != "rule" {
			continue
		}
		b, err := os.ReadFile(it.Path)
		if err != nil {
			continue
		}
		text := string(b)
		f := Finding{Path: it.Path}
		var removed strings.Builder
		lines := 0
		for _, r := range runs(text, func(n string) bool { return seen[n] }) {
			if strings.Count(text, r.raw) != 1 {
				continue // the edit must point at exactly one place
			}
			old := withBlankAfter(text, r.raw)
			f.Edits = append(f.Edits, Edit{Path: it.Path, Old: old, New: ""})
			f.Diff += hunk(it.Path, lineOf(text, r.raw)) + diffLines("-", r.raw)
			removed.WriteString(r.raw)
			lines += len(r.norms)
		}
		if lines == 0 {
			continue
		}
		f.Saves = analytics.EstimateTokens(removed.String())
		f.Diff = strings.TrimRight(f.Diff, "\n")
		f.Title = fmt.Sprintf("%s repeats %d lines your own CLAUDE.md already sends", it.Name, lines)
		f.Detail = "Every session of this project gets these lines twice. Remove the project copy; " +
			"your user-level files keep sending them."
		f.Params = map[string]string{"name": it.Name, "lines": fmt.Sprint(lines), "count": fmt.Sprint(lines), "tokens": fmt.Sprint(f.Saves)}
		out = append(out, f)
	}
	return out
}

// projectClaudeMDs returns the CLAUDE.md files that belong to project p itself (not to its
// parent folders), with their contents.
func projectClaudeMDs(items []claudecode.Item, p string) map[string]string {
	files := map[string]string{}
	for _, it := range items {
		if it.Kind == "CLAUDE.md" && it.Scope == "project" && strings.HasPrefix(it.Path, p+string(filepath.Separator)) {
			if b, err := os.ReadFile(it.Path); err == nil {
				files[it.Path] = string(b)
			}
		}
	}
	return files
}

// sharedBlock flags a block of CLAUDE.md that several projects carry, and proposes to keep one
// copy in the user's CLAUDE.md. It reports the same finding from every project involved, so the
// UI shows it once with all of them.
func sharedBlock(in Input) []Finding {
	if len(in.Projects) < minSharedCopies {
		return nil
	}
	user := userLines(in.Items)
	projects := sortedProjects(in.Projects)
	files := map[string]map[string]string{} // project → path → text
	where := map[string]map[string]bool{}   // normalized line → projects that have it
	for _, p := range projects {
		files[p] = projectClaudeMDs(in.Projects[p], p)
		for _, text := range files[p] {
			for _, line := range strings.Split(text, "\n") {
				if n := norm(line); n != "" {
					if where[n] == nil {
						where[n] = map[string]bool{}
					}
					where[n][p] = true
				}
			}
		}
	}

	var out []Finding
	paths := sortedKeys(files[in.Project])
	for _, path := range paths {
		text := files[in.Project][path]
		blocks := runs(text, func(n string) bool { return len(where[n]) >= minSharedCopies && !user[n] })
		for _, b := range blocks {
			if len(b.norms) == 1 && len(b.norms[0]) < minSingleBlock {
				continue
			}
			// Projects that carry the whole block, in a stable order.
			var holders []string
			for _, p := range projects {
				for _, t := range files[p] {
					if _, ok := findRun(t, b.norms); ok {
						holders = append(holders, p)
						break
					}
				}
			}
			sort.Strings(holders)
			if len(holders) < minSharedCopies {
				continue
			}
			if f, ok := moveToUser(in, files, holders, b); ok {
				out = append(out, f)
			}
		}
	}
	return out
}

// moveToUser builds the fix for a shared block: append it to the user's CLAUDE.md and remove it
// from every project that has it.
func moveToUser(in Input, files map[string]map[string]string, holders []string, b run) (Finding, bool) {
	target := claudecode.UserClaudeMD(in.ConfigDir)
	var edits []Edit
	var block, removals string
	for _, p := range holders {
		for _, path := range sortedKeys(files[p]) {
			raw, ok := findRun(files[p][path], b.norms)
			if !ok || strings.Count(files[p][path], raw) != 1 {
				continue
			}
			raw = withHeading(files[p][path], raw)
			if block == "" {
				block = raw
			}
			removals += hunk(path, lineOf(files[p][path], raw)) + diffLines("-", raw)
			raw = withBlankAfter(files[p][path], raw)
			edits = append(edits, Edit{Path: path, Old: raw, New: ""})
			break
		}
	}
	if len(edits) < minSharedCopies {
		return Finding{}, false
	}
	tokens := analytics.EstimateTokens(block)
	others := len(in.Projects) - len(holders)
	first := strings.TrimLeft(b.norms[0], "-*+ ") // quoted without the list marker
	if len([]rune(first)) > 48 {
		first = string([]rune(first)[:47]) + "…"
	}
	state := "some" // other projects would start getting the block
	if others == 0 {
		state = "all"
	}
	return Finding{
		Title: fmt.Sprintf("%d lines are copied into %d projects: %s", len(b.norms), len(holders), first),
		Detail: fmt.Sprintf("Each copy has to be kept up to date by hand. Keep one copy in your CLAUDE.md "+
			"instead; it is then also sent in the %d other projects of this profile (≈ %d tokens each).", others, tokens),
		Path:  target,
		Diff:  strings.TrimRight(hunk(target, appendLine(target))+diffLines("+", block)+removals, "\n"),
		Edits: append([]Edit{{Path: target, Append: block}}, edits...),
		Params: map[string]string{"lines": fmt.Sprint(len(b.norms)), "projects": fmt.Sprint(len(holders)),
			"count": fmt.Sprint(len(holders)), "first": first, "others": fmt.Sprint(others),
			"tokens": fmt.Sprint(tokens), "state": state},
	}, true
}

// copiedExtension flags a skill, rule or agent that is identical in several projects. A skill is
// its whole folder (SKILL.md and any scripts or references), a rule or agent is one file. When
// the user's settings have nothing of that name yet, the fix moves one copy there and deletes the
// others; otherwise it is left to the user.
func copiedExtension(in Input) []Finding {
	if len(in.Projects) < 2 {
		return nil
	}
	type copyOf struct {
		kind, name string
		roots      []string // the skill folders or the rule/agent files
		files      map[string]string
	}
	groups := map[string]*copyOf{}
	seen := map[string]bool{} // a file can be listed more than once
	for _, p := range sortedProjects(in.Projects) {
		for _, it := range in.Projects[p] {
			if it.Scope != "project" || (it.Kind != "skill" && it.Kind != "rule" && it.Kind != "agent") || seen[it.Path] {
				continue
			}
			seen[it.Path] = true
			root := it.Path
			if it.Kind == "skill" {
				root = filepath.Dir(it.Path)
			}
			files, sum, ok := extensionFiles(root)
			if !ok {
				continue
			}
			key := it.Kind + "\x00" + filepath.Base(root) + "\x00" + sum
			if groups[key] == nil {
				groups[key] = &copyOf{kind: it.Kind, name: it.Name, files: files}
			}
			groups[key].roots = append(groups[key].roots, root)
		}
	}
	var out []Finding
	for _, g := range groups {
		if len(g.roots) < 2 || !hasPathIn(g.roots, in.Project) {
			continue
		}
		sort.Strings(g.roots)
		f := Finding{
			Title: fmt.Sprintf("%s %q is copied into %d projects", g.kind, g.name, len(g.roots)),
			Params: map[string]string{"kind": g.kind, "name": g.name, "copies": fmt.Sprint(len(g.roots)),
				"count": fmt.Sprint(len(g.roots)), "state": g.kind},
		}
		target := filepath.Join(claudecode.UserExtensionDir(in.ConfigDir, g.kind), filepath.Base(g.roots[0]))
		if _, err := os.Stat(target); err == nil {
			f.Params["state"] = "exists" // the user's settings already have one of that name
			f.Detail = "Your own settings already have a " + g.kind + " of this name, so quipu does not move it; " +
				"compare them and keep one."
		} else {
			f.Detail = "The copies are identical. Keep one in your own settings, where it works in every project, " +
				"and delete the others, so a change is made once."
			f.Path = target
			f.Edits, f.Diff = moveExtension(g.roots, g.files, target)
		}
		out = append(out, f)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Title < out[j].Title })
	return out
}

// extensionFiles reads a skill folder or a single file: relative path → content, and a hash of
// all of it. Folders bigger than a skill would be are skipped.
func extensionFiles(root string) (map[string]string, string, bool) {
	files := map[string]string{}
	size := 0
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		size += len(b)
		rel, _ := filepath.Rel(root, path)
		files[rel] = string(b)
		return nil
	})
	if err != nil || size > 1<<20 || len(files) == 0 {
		return nil, "", false
	}
	h := sha256.New()
	for _, rel := range sortedKeys(files) {
		fmt.Fprintf(h, "%s\x00%d\x00%s", rel, len(files[rel]), files[rel])
	}
	return files, fmt.Sprintf("%x", h.Sum(nil)), true
}

// moveExtension creates the files under target and deletes every copy, pruning the folders it
// empties up to the project's skills/rules/agents folder.
func moveExtension(roots []string, files map[string]string, target string) ([]Edit, string) {
	var edits []Edit
	var diff strings.Builder
	single := len(files) == 1 && files["."] != "" // a rule or agent: root is the file itself
	place := func(root, rel string) string {
		if single {
			return root
		}
		return filepath.Join(root, rel)
	}
	for _, rel := range sortedKeys(files) {
		path := place(target, rel)
		edits = append(edits, Edit{Path: path, Create: files[rel]})
		diff.WriteString(hunk(path, 1) + diffPreview("+", files[rel]))
	}
	for _, root := range roots {
		for _, rel := range sortedKeys(files) {
			path := place(root, rel)
			edits = append(edits, Edit{Path: path, Delete: true, Old: files[rel], Stop: filepath.Dir(root)})
			diff.WriteString(hunk(path, 1) + diffPreview("-", files[rel]))
		}
	}
	return edits, strings.TrimRight(diff.String(), "\n")
}

// diffPreview shows the first lines of a whole file being created or deleted.
func diffPreview(sign, text string) string {
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	if len(lines) > 8 {
		lines = append(lines[:7], "…")
	}
	return diffLines(sign, strings.Join(lines, "\n"))
}

func hasPathIn(paths []string, project string) bool {
	for _, p := range paths {
		if strings.HasPrefix(p, project+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func sortedProjects(m map[string][]claudecode.Item) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
