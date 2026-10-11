package claudecode

import "testing"

func TestIsSession(t *testing.T) {
	tests := []struct {
		args    []string
		session bool
		rc      bool
	}{
		{[]string{"claude"}, true, false},
		{[]string{"/home/u/.local/bin/claude", "--resume"}, true, false},
		{[]string{"claude", "remote-control"}, true, true},
		{[]string{"claude", "--rc"}, true, true},
		{[]string{"node", "/usr/lib/node_modules/@anthropic-ai/claude-code/cli.js", "--remote-control"}, true, true},
		{[]string{"/home/u/.local/bin/claude", "--chrome-native-host"}, false, false},
		{[]string{"claude", "mcp", "serve"}, false, false},
		{[]string{"claude", "doctor"}, false, false},
		{[]string{"node", "server.js"}, false, false},
		{[]string{"claudette"}, false, false},
		{nil, false, false},
	}
	for _, tt := range tests {
		if got := isSession(tt.args); got != tt.session {
			t.Errorf("isSession(%q) = %v, want %v", tt.args, got, tt.session)
		}
		if tt.session {
			if got := remoteControlFlag(tt.args); got != tt.rc {
				t.Errorf("remoteControlFlag(%q) = %v, want %v", tt.args, got, tt.rc)
			}
		}
	}
}

func TestRunningSessionsDoesNotFail(t *testing.T) {
	// Whatever runs on the test machine, listing must work and find no session in this test process.
	list, err := RunningSessions()
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range list {
		if p.PID <= 0 {
			t.Errorf("bad process %+v", p)
		}
	}
}
