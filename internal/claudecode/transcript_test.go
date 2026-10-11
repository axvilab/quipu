package claudecode

import (
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
