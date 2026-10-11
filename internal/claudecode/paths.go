// Package claudecode holds all knowledge about Claude Code internals: file layout,
// project directory naming, credential storage. These details are undocumented and
// may change between Claude Code versions, so they must not leak into other packages.
package claudecode

import "strings"

// ProjectDirName returns the name Claude Code uses for a project's directory under
// <config>/projects: every character that is not an ASCII letter or digit becomes "-".
//
// The mapping is lossy ("/a_b" and "/a/b" both give "-a-b"), so it can only be used
// in this direction; resolving a directory name back to a path needs other evidence.
func ProjectDirName(projectPath string) string {
	var b strings.Builder
	b.Grow(len(projectPath))
	for _, r := range projectPath {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	return b.String()
}
