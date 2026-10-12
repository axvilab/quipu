package claudecode

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// SkillListing is the list of skills Claude Code showed the model at the start of a session, as
// it records it in the transcript. It is what Claude Code really loaded, so quipu checks its own
// reading of the files against it.
type SkillListing struct {
	At time.Time
	// Skills maps each listed name to its text in the listing. Names are as Claude Code writes
	// them: "ship" for the user's and the project's skills and its own, "<plugin>:<skill>" for a
	// plugin's, "anthropic-skills:<skill>" for claude.ai's.
	Skills map[string]string
}

// listingFiles bounds how many of a project's newest transcripts are searched for a listing.
const listingFiles = 5

// LastSkillListing returns the listing of the newest session of the project that has one.
func LastSkillListing(configDir, projectPath string) (SkillListing, bool) {
	files, _ := filepath.Glob(filepath.Join(configDir, "projects", ProjectDirName(projectPath), "*.jsonl"))
	type file struct {
		path string
		mod  time.Time
	}
	var byAge []file
	for _, f := range files {
		if fi, err := os.Stat(f); err == nil {
			byAge = append(byAge, file{f, fi.ModTime()})
		}
	}
	sort.Slice(byAge, func(i, j int) bool { return byAge[i].mod.After(byAge[j].mod) })
	for i, f := range byAge {
		if i == listingFiles {
			break
		}
		if l, ok := readSkillListing(f.path); ok {
			return l, true
		}
	}
	return SkillListing{}, false
}

// readSkillListing finds the first full skill listing in a transcript.
func readSkillListing(path string) (SkillListing, bool) {
	f, err := os.Open(path)
	if err != nil {
		return SkillListing{}, false
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<20), maxLineSize)
	marker := []byte(`"skill_listing"`)
	for sc.Scan() {
		if !bytes.Contains(sc.Bytes(), marker) {
			continue // most lines are large and not listings: skip them without decoding
		}
		var l struct {
			Timestamp  time.Time `json:"timestamp"`
			Attachment struct {
				Type      string   `json:"type"`
				IsInitial bool     `json:"isInitial"`
				Names     []string `json:"names"`
				Content   string   `json:"content"`
			} `json:"attachment"`
		}
		if json.Unmarshal(sc.Bytes(), &l) != nil {
			continue
		}
		a := l.Attachment
		if a.Type != "skill_listing" || !a.IsInitial || len(a.Names) == 0 {
			continue
		}
		return SkillListing{At: l.Timestamp, Skills: listingTexts(a.Names, a.Content)}, true
	}
	return SkillListing{}, false
}

// listingTexts splits a listing ("- name: description" per skill, descriptions may wrap) into
// each name's text.
func listingTexts(names []string, content string) map[string]string {
	out := map[string]string{}
	known := map[string]bool{}
	for _, n := range names {
		known[n] = true
		out[n] = n
	}
	current := ""
	for _, line := range strings.Split(content, "\n") {
		if rest, ok := strings.CutPrefix(line, "- "); ok {
			if name, _, ok := strings.Cut(rest, ":"); ok && known[name] {
				current = name
				out[name] = rest
				continue
			}
			// "plugin:skill: description": the name itself has a colon.
			if i := strings.Index(rest, ": "); i > 0 && known[rest[:i]] {
				current = rest[:i]
				out[current] = rest
				continue
			}
		}
		if current != "" {
			out[current] += "\n" + line
		}
	}
	return out
}

// ListedName is how Claude Code names an item in its skill listing, "" for what it does not list.
func ListedName(it Item) string {
	if it.Kind != "skill" {
		return ""
	}
	switch {
	case it.Scope == "claude.ai":
		return "anthropic-skills:" + it.Name
	case strings.HasPrefix(it.Scope, "plugin "):
		return strings.TrimPrefix(it.Scope, "plugin ") + ":" + it.Name
	}
	return it.Name
}
