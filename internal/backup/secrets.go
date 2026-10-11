package backup

import (
	"regexp"
	"strings"
)

// Secret is something that looks like a credential in a backed-up file.
type Secret struct {
	Path string `json:"path"`
	Line int    `json:"line"`
	Kind string `json:"kind"`
}

// secretPatterns match credentials of common services and generic key/token values. They err on
// the side of reporting: a false alarm costs a look, a leaked key costs much more.
var secretPatterns = []struct {
	kind string
	re   *regexp.Regexp
}{
	{"anthropic-key", regexp.MustCompile(`sk-ant-[A-Za-z0-9_-]{20,}`)},
	{"openai-key", regexp.MustCompile(`\bsk-(proj-)?[A-Za-z0-9_-]{32,}`)},
	{"github-token", regexp.MustCompile(`\b(ghp|gho|ghu|ghs|ghr)_[A-Za-z0-9]{30,}|github_pat_[A-Za-z0-9_]{40,}`)},
	{"slack-token", regexp.MustCompile(`\bxox[abposr]-[A-Za-z0-9-]{10,}`)},
	{"aws-key", regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`)},
	{"google-key", regexp.MustCompile(`\bAIza[0-9A-Za-z_-]{35}\b`)},
	{"private-key", regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`)},
	{"jwt", regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{10,}\.eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}`)},
	// "API_KEY": "...", token = '...', password: ... with a value long enough to be real.
	{"credential-value", regexp.MustCompile(`(?i)["']?[a-z0-9_]*(api[_-]?key|secret|token|password|passwd)[a-z0-9_]*["']?\s*[:=]\s*["']?[^\s"',}{]{12,}`)},
}

// Scan reports what looks like a credential in the entries, by file and line.
func Scan(entries []Entry) []Secret {
	var out []Secret
	for _, e := range entries {
		for i, line := range strings.Split(string(e.Data), "\n") {
			for _, p := range secretPatterns {
				if p.re.MatchString(line) {
					out = append(out, Secret{Path: e.Path, Line: i + 1, Kind: p.kind})
					break
				}
			}
		}
	}
	return out
}
