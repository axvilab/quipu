package claudecode

import (
	"strings"

	"os"
	"testing"
)

func TestReadResponses(t *testing.T) {
	f, err := os.Open("testdata/session.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	got, stats, err := ReadResponses(f)
	if err != nil {
		t.Fatal(err)
	}

	wantStats := ReadStats{Lines: 11, Malformed: 1, Repeated: 1, Synthetic: 1}
	if stats != wantStats {
		t.Errorf("stats = %+v, want %+v", stats, wantStats)
	}

	if len(got) != 3 {
		t.Fatalf("got %d responses, want 3 (msg_1 split over two lines must count once)", len(got))
	}
	for i, id := range []string{"msg_1", "msg_2", "msg_3"} {
		if got[i].MessageID != id {
			t.Errorf("response %d: id %q, want %q", i, got[i].MessageID, id)
		}
	}

	first := got[0]
	if first.Usage.CacheCreationTokens != 20000 || first.Usage.CacheCreation.Ephemeral1h != 20000 {
		t.Errorf("msg_1 usage = %+v", first.Usage)
	}
	if first.Usage.PromptTokens() != 20003 {
		t.Errorf("msg_1 prompt tokens = %d, want 20003", first.Usage.PromptTokens())
	}
	if first.SessionID != "s-1" || first.Cwd != "/srv/work/demo" || first.Version != "2.1.290" {
		t.Errorf("msg_1 metadata = %+v", first)
	}
	if first.Timestamp.IsZero() {
		t.Error("msg_1 timestamp not parsed")
	}

	if got[1].CacheMiss != nil {
		t.Errorf("msg_2 cache miss = %+v, want nil", got[1].CacheMiss)
	}
	miss := got[2].CacheMiss
	if miss == nil || miss.Type != "model_changed" || miss.MissedTokens != 20500 {
		t.Errorf("msg_3 cache miss = %+v", miss)
	}
}

func TestReadResponsesPlugins(t *testing.T) {
	// One message written as three lines (one content block each), then a slash command.
	lines := []string{
		`{"type":"assistant","sessionId":"s","message":{"id":"m1","model":"claude-opus-5-5","content":[{"type":"text","text":"Let me look."}],"usage":{"input_tokens":1}}}`,
		`{"type":"assistant","sessionId":"s","message":{"id":"m1","model":"claude-opus-5-5","content":[{"type":"tool_use","name":"mcp__plugin_playwright_playwright__browser_click","input":{}}],"usage":{"input_tokens":1}}}`,
		`{"type":"assistant","sessionId":"s","message":{"id":"m1","model":"claude-opus-5-5","content":[{"type":"tool_use","name":"Skill","input":{"skill":"frontend-design:frontend-design"}}],"usage":{"input_tokens":1}}}`,
		`{"type":"user","sessionId":"s","message":{"role":"user","content":"<command-name>/ship:release</command-name>"}}`,
		`{"type":"assistant","sessionId":"s","message":{"id":"m2","model":"claude-opus-5-5","content":[{"type":"tool_use","name":"Agent","input":{"subagent_type":"Explore"}}],"usage":{"input_tokens":1}}}`,
		`{"type":"user","sessionId":"s","message":{"role":"user","content":[{"type":"tool_result","content":"ok"}]}}`,
		`{"type":"assistant","sessionId":"s","message":{"id":"m3","model":"claude-opus-5-5","content":[{"type":"tool_use","name":"Agent","input":{"subagent_type":"pr-tools:reviewer"}}],"usage":{"input_tokens":1}}}`,
	}
	rs, stats, err := ReadResponses(strings.NewReader(strings.Join(lines, "\n")))
	if err != nil {
		t.Fatal(err)
	}
	if len(rs) != 3 || stats.Repeated != 2 {
		t.Fatalf("%d responses, %d repeated", len(rs), stats.Repeated)
	}
	want := [][]string{{"frontend-design", "playwright"}, {"ship"}, {"pr-tools"}}
	for i, w := range want {
		if strings.Join(rs[i].Plugins, ",") != strings.Join(w, ",") {
			t.Errorf("response %d plugins = %v, want %v", i, rs[i].Plugins, w)
		}
	}
	// Skills: what Claude invoked, and the command the user typed before a response.
	wantSkills := [][]string{{"frontend-design:frontend-design"}, {"ship:release"}, nil}
	for i, w := range wantSkills {
		if strings.Join(rs[i].Skills, ",") != strings.Join(w, ",") {
			t.Errorf("response %d skills = %v, want %v", i, rs[i].Skills, w)
		}
	}
}
