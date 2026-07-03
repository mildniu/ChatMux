package tmux

import (
	"testing"
	"time"
)

func TestParseSessionsAppliesPaneScan(t *testing.T) {
	output := "__chatmux_now\t1710000040\n" +
		"session\t$0\tdev\t2\t1\t1710000039\tnode\t0\t\t1709999000\n" +
		"window\tdev\t@0\t0\tagent\t1\t1710000039\tnode\t0\t\t1\t190\t45\tbinjie09\n" +
		"window\tdev\t@1\t1\tshell\t0\t1710000000\tzsh\t0\t\t1\t190\t45\tbinjie09\n" +
		"__chatmux_screen\t@0\t/dev/pts/7\t574367583 831\n" +
		"__chatmux_pscan\tpts/7\tnode /home/user/.nvm/versions/node/v24.14.1/bin/codex\n" +
		"__chatmux_pscan\tpts/9\t/usr/bin/vim main.go\n" +
		"__chatmux_pscan\t?\tnode /srv/codex\n"
	sessions, err := ParseSessionsAt(output, time.Unix(2000000000, 0).UTC())
	if err != nil {
		t.Fatalf("ParseSessions failed: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	agent := sessions[0].WindowList[0]
	if agent.ProcessName != "codex" {
		t.Fatalf("expected codex relabel via tty processes, got %q", agent.ProcessName)
	}
	if agent.ScreenHash != "574367583 831" {
		t.Fatalf("expected screen hash to be attached, got %q", agent.ScreenHash)
	}
	if sessions[0].ProcessName != "codex" {
		t.Fatalf("expected session process aggregation to surface codex, got %q", sessions[0].ProcessName)
	}
	shell := sessions[0].WindowList[1]
	if shell.ScreenHash != "" || shell.ProcessName != "zsh" {
		t.Fatalf("expected shell window untouched, got %#v", shell)
	}
}

func TestAgentFromProcessArgs(t *testing.T) {
	cases := []struct {
		args string
		want string
	}{
		{"node /home/user/.nvm/versions/node/v24.14.1/bin/codex", "codex"},
		{"/vendor/x86_64-unknown-linux-musl/bin/codex", "codex"},
		{"-zsh", ""},
		{"node /srv/my-codex-clone/server.js", ""},
		{"npm run dev", ""},
		{"", ""},
	}
	for _, testCase := range cases {
		if got := agentFromProcessArgs(testCase.args); got != testCase.want {
			t.Fatalf("agentFromProcessArgs(%q) = %q, want %q", testCase.args, got, testCase.want)
		}
	}
}

func TestListSessionsCommandIncludesPaneScan(t *testing.T) {
	command := ListSessionsCommand()
	for _, fragment := range []string{"capture-pane", "cksum", "__chatmux_screen", "__chatmux_pscan", "pane_tty"} {
		if !containsLoginShellFragment(command, fragment) {
			t.Fatalf("expected list command to contain %q, got %q", fragment, command)
		}
	}
}
