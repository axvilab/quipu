package claudecode

import "strings"

// frontmatter holds the few fields quipu reads from the YAML header of skills, agents and rules.
type frontmatter struct {
	Name        string
	Description string
	Paths       []string
	// NoModel is disable-model-invocation: true. Claude Code then keeps the skill off the list it
	// shows the model; only the user runs it, as /name.
	NoModel bool
}

// parseFrontmatter splits a Markdown file into its YAML header and body.
//
// It understands only what these files use in practice: "key: value" (optionally quoted),
// block scalars ("key: >" or "key: |" followed by indented lines) and lists ("paths:" followed
// by "- item" lines, or an inline "[a, b]"). Anything else is ignored. A file without a header
// returns the whole text as body.
func parseFrontmatter(text string) (frontmatter, string) {
	var fm frontmatter
	rest, ok := strings.CutPrefix(text, "---\n")
	if !ok {
		return fm, text
	}
	header, body, ok := strings.Cut(rest, "\n---")
	if !ok {
		return fm, text
	}
	body = strings.TrimPrefix(strings.TrimPrefix(body, "\n"), "\r")
	if i := strings.IndexByte(body, '\n'); i >= 0 && strings.TrimSpace(body[:i]) == "" {
		body = body[i+1:]
	}

	lines := strings.Split(header, "\n")
	for i := 0; i < len(lines); i++ {
		key, value, ok := strings.Cut(lines[i], ":")
		if !ok || strings.HasPrefix(key, " ") || strings.HasPrefix(key, "-") {
			continue
		}
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)

		// Collect the indented continuation lines of this key.
		var block []string
		for i+1 < len(lines) && (strings.HasPrefix(lines[i+1], " ") || strings.HasPrefix(lines[i+1], "\t")) {
			i++
			block = append(block, strings.TrimSpace(lines[i]))
		}

		switch key {
		case "name":
			fm.Name = unquote(value)
		case "description":
			if value == ">" || value == "|" || value == ">-" || value == "|-" || value == "" {
				fm.Description = strings.Join(block, " ")
			} else {
				fm.Description = unquote(value)
			}
		case "paths":
			fm.Paths = parseList(value, block)
		case "disable-model-invocation":
			fm.NoModel = unquote(value) == "true"
		}
	}
	return fm, body
}

func parseList(value string, block []string) []string {
	var out []string
	if inner, ok := strings.CutPrefix(value, "["); ok {
		for _, item := range strings.Split(strings.TrimSuffix(inner, "]"), ",") {
			if item = unquote(strings.TrimSpace(item)); item != "" {
				out = append(out, item)
			}
		}
		return out
	}
	if value != "" {
		return []string{unquote(value)}
	}
	for _, line := range block {
		if item, ok := strings.CutPrefix(line, "-"); ok {
			out = append(out, unquote(strings.TrimSpace(item)))
		}
	}
	return out
}

func unquote(s string) string {
	if len(s) >= 2 && (s[0] == '"' && s[len(s)-1] == '"' || s[0] == '\'' && s[len(s)-1] == '\'') {
		return s[1 : len(s)-1]
	}
	return s
}
