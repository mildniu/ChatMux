package tmux

import (
	"testing"
	"time"
)

func TestSessionStatus(t *testing.T) {
	now := time.Unix(1710000040, 0).UTC()
	recent := time.Unix(1710000038, 0).UTC()
	stale := time.Unix(1710000000, 0).UTC()
	cases := []struct {
		name  string
		input sessionStatusInput
		want  string
	}{
		{"dead pane clean exit", sessionStatusInput{currentCommand: "sh", now: now, paneDead: true, paneDeadStatus: "0", updatedAt: recent}, SessionStatusDone},
		{"dead pane failure", sessionStatusInput{currentCommand: "sh", now: now, paneDead: true, paneDeadStatus: "2", updatedAt: recent}, SessionStatusFailed},
		{"dead pane unparseable exit", sessionStatusInput{currentCommand: "sh", now: now, paneDead: true, paneDeadStatus: "x", updatedAt: recent}, SessionStatusUnknown},
		{"empty command", sessionStatusInput{currentCommand: "", now: now, updatedAt: recent}, SessionStatusUnknown},
		{"shell with recent activity is idle", sessionStatusInput{currentCommand: "zsh", now: now, updatedAt: recent}, SessionStatusIdle},
		{"login shell prefix is idle", sessionStatusInput{currentCommand: "-zsh", now: now, updatedAt: recent}, SessionStatusIdle},
		{"shell path is idle", sessionStatusInput{currentCommand: "/bin/bash", now: now, updatedAt: recent}, SessionStatusIdle},
		{"shell keeps stale agent title", sessionStatusInput{currentCommand: "zsh", now: now, paneTitle: "✳ Claude Code", updatedAt: recent}, SessionStatusIdle},
		{"program with recent activity", sessionStatusInput{currentCommand: "node", now: now, updatedAt: recent}, SessionStatusRunning},
		{"program gone quiet", sessionStatusInput{currentCommand: "node", now: now, updatedAt: stale}, SessionStatusWaiting},
		{"spinner title while quiet", sessionStatusInput{currentCommand: "claude", now: now, paneTitle: "⠐ fix bug", updatedAt: stale}, SessionStatusRunning},
		{"asterisk title despite recent repaint", sessionStatusInput{currentCommand: "claude", now: now, paneTitle: "✳ fix bug", updatedAt: recent}, SessionStatusWaiting},
		{"alternate asterisk title", sessionStatusInput{currentCommand: "claude", now: now, paneTitle: "✻ fix bug", updatedAt: recent}, SessionStatusWaiting},
		{"plain title falls back to activity", sessionStatusInput{currentCommand: "claude", now: now, paneTitle: "binjie09", updatedAt: recent}, SessionStatusRunning},
		{"blank braille cell is not a spinner", sessionStatusInput{currentCommand: "claude", now: now, paneTitle: "⠀ topic", updatedAt: stale}, SessionStatusWaiting},
		{"future activity is not recent", sessionStatusInput{currentCommand: "node", now: now, updatedAt: now.Add(time.Minute)}, SessionStatusWaiting},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := sessionStatus(testCase.input); got != testCase.want {
				t.Fatalf("sessionStatus = %q, want %q", got, testCase.want)
			}
		})
	}
}

func TestAggregateSessionStatus(t *testing.T) {
	cases := []struct {
		name        string
		windows     []Window
		wantStatus  string
		wantProcess string
	}{
		{"no windows", nil, SessionStatusUnknown, ""},
		{
			"running beats waiting",
			[]Window{
				{Status: SessionStatusWaiting, ProcessName: "claude"},
				{Status: SessionStatusRunning, ProcessName: "codex"},
			},
			SessionStatusRunning, "codex",
		},
		{
			"waiting beats idle",
			[]Window{
				{Status: SessionStatusIdle, ProcessName: "zsh"},
				{Status: SessionStatusWaiting, ProcessName: "claude"},
			},
			SessionStatusWaiting, "claude",
		},
		{
			"failed beats done and idle",
			[]Window{
				{Status: SessionStatusDone, ProcessName: "sh"},
				{Status: SessionStatusFailed, ProcessName: "sh"},
				{Status: SessionStatusIdle, ProcessName: "zsh"},
			},
			SessionStatusFailed, "sh",
		},
		{
			"all idle",
			[]Window{
				{Status: SessionStatusIdle, ProcessName: "zsh"},
				{Status: SessionStatusIdle, ProcessName: "bash"},
			},
			SessionStatusIdle, "zsh",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			status, process := AggregateSessionStatus(testCase.windows)
			if status != testCase.wantStatus || process != testCase.wantProcess {
				t.Fatalf("AggregateSessionStatus = (%q, %q), want (%q, %q)",
					status, process, testCase.wantStatus, testCase.wantProcess)
			}
		})
	}
}

func TestSessionAggregationOverridesSessionLine(t *testing.T) {
	// The session line's active pane is a shell, but a background window runs
	// claude with a working spinner title: the session must surface claude.
	output := "__chatmux_now\t1710000040\n" +
		"session\t$0\tdev\t2\t1\t1710000039\tzsh\t0\t\t1709999000\n" +
		"window\tdev\t@0\t0\tshell\t1\t1710000039\tzsh\t0\t\t1\t190\t45\tbinjie09\n" +
		"window\tdev\t@1\t1\tagent\t0\t1710000010\tclaude\t0\t\t1\t190\t45\t⠂ refactor status\n"
	sessions, err := ParseSessionsAt(output, time.Unix(2000000000, 0).UTC())
	if err != nil {
		t.Fatalf("ParseSessions failed: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].Status != SessionStatusRunning || sessions[0].ProcessName != "claude" {
		t.Fatalf("expected claude running session, got %q/%q", sessions[0].Status, sessions[0].ProcessName)
	}
	if sessions[0].WindowList[0].Status != SessionStatusIdle {
		t.Fatalf("expected idle shell window, got %q", sessions[0].WindowList[0].Status)
	}
	if sessions[0].WindowList[1].Status != SessionStatusRunning {
		t.Fatalf("expected running claude window, got %q", sessions[0].WindowList[1].Status)
	}
}
