package claudecode

import "testing"

func TestProjectDirName(t *testing.T) {
	// Synthetic paths following the naming observed in real ~/.claude/projects directories.
	cases := map[string]string{
		"/home/alice":         "-home-alice",
		"/srv/work/billing2":  "-srv-work-billing2",
		"/srv/work/api_v2":    "-srv-work-api-v2",
		"/srv/pet/log.parser": "-srv-pet-log-parser",
		"/srv/work/MyService": "-srv-work-MyService",
	}
	for in, want := range cases {
		if got := ProjectDirName(in); got != want {
			t.Errorf("ProjectDirName(%q) = %q, want %q", in, got, want)
		}
	}
}
