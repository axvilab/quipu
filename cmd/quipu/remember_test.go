package main

import (
	"bufio"
	"io"
	"strings"
	"testing"

	"github.com/axvilab/quipu/internal/profile"
)

func TestAskProfileAndRemember(t *testing.T) {
	c := &profile.Config{Profiles: []profile.Profile{{Name: "default"}, {Name: "work"}}}
	// One reader for both questions, as runClaudeWith does: nothing typed is lost between them.
	in := bufio.NewReader(strings.NewReader("2\n\n"))
	if got := askProfile(c, in, io.Discard); got != "work" {
		t.Fatalf("askProfile = %q", got)
	}
	if !askRemember("work", "~/w", in, io.Discard) {
		t.Error("Enter should mean yes")
	}
	for answer, want := range map[string]bool{"n\n": false, "no\n": false, "y\n": true, "так\n": true, "ні\n": false} {
		if got := askRemember("work", "~/w", bufio.NewReader(strings.NewReader(answer)), io.Discard); got != want {
			t.Errorf("answer %q = %v, want %v", answer, got, want)
		}
	}
}
