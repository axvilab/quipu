package claudecode

import (
	"reflect"
	"testing"
)

func TestParseFrontmatter(t *testing.T) {
	cases := []struct {
		name, in string
		want     frontmatter
		body     string
	}{
		{
			name: "no header",
			in:   "# Title\ntext\n",
			body: "# Title\ntext\n",
		},
		{
			name: "simple and quoted",
			in:   "---\nname: adr-new\ndescription: \"Creates an ADR: use it\"\nargument-hint: [title]\n---\n\nBody\n",
			want: frontmatter{Name: "adr-new", Description: "Creates an ADR: use it"},
			body: "Body\n",
		},
		{
			name: "block scalar description",
			in:   "---\nname: x\ndescription: >\n  first line\n  second line\nmodel: sonnet\n---\nBody",
			want: frontmatter{Name: "x", Description: "first line second line"},
			body: "Body",
		},
		{
			name: "paths list",
			in:   "---\npaths:\n  - \"**/*.go\"\n  - internal/**\n---\nRule",
			want: frontmatter{Paths: []string{"**/*.go", "internal/**"}},
			body: "Rule",
		},
		{
			name: "paths inline",
			in:   "---\npaths: [\"src/**\", tests/**]\n---\nRule",
			want: frontmatter{Paths: []string{"src/**", "tests/**"}},
			body: "Rule",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, body := parseFrontmatter(c.in)
			if !reflect.DeepEqual(got, c.want) || body != c.body {
				t.Errorf("got %+v body %q, want %+v body %q", got, body, c.want, c.body)
			}
		})
	}
}
