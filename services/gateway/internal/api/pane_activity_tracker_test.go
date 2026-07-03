package api

import (
	"testing"
	"time"

	"github.com/chatmux/chatmux/services/gateway/internal/tmux"
)

type screenProbe struct {
	hash   string
	status string
	title  string
	width  int
}

func screenProbeSessions(probe screenProbe) []tmux.Session {
	windows := []tmux.Window{{
		ID: "@1", Index: 0, Status: probe.status, ProcessName: "codex",
		PaneTitle: probe.title, ScreenHash: probe.hash, Width: probe.width, Height: 40,
	}}
	status, processName := tmux.AggregateSessionStatus(windows)
	return []tmux.Session{{
		Name: "dev", Status: status, ProcessName: processName, WindowList: windows,
	}}
}

func TestTrackerStableScreenStaysWaitingThroughRepaintActivity(t *testing.T) {
	tracker := newPaneActivityTracker()
	base := time.Unix(1710000000, 0).UTC()
	tracker.Apply("host-1", screenProbeSessions(screenProbe{hash: "h1", status: "waiting", width: 120}), base)

	// Attach/focus repaint bumped window_activity ("running"), but the screen
	// content is identical: the fingerprint keeps it waiting.
	flapped := screenProbeSessions(screenProbe{hash: "h1", status: "running", width: 120})
	tracker.Apply("host-1", flapped, base.Add(2*time.Second))
	if flapped[0].Status != tmux.SessionStatusWaiting || flapped[0].WindowList[0].Status != tmux.SessionStatusWaiting {
		t.Fatalf("expected identical screen to stay waiting, got session=%q window=%q",
			flapped[0].Status, flapped[0].WindowList[0].Status)
	}
}

func TestTrackerChangingScreenIsRunningAndSettles(t *testing.T) {
	tracker := newPaneActivityTracker()
	base := time.Unix(1710000000, 0).UTC()
	tracker.Apply("host-1", screenProbeSessions(screenProbe{hash: "h1", status: "waiting", width: 120}), base)

	// Screen content changes (spinner/stream): running, even if the activity
	// heuristic already decayed to waiting.
	working := screenProbeSessions(screenProbe{hash: "h2", status: "waiting", width: 120})
	tracker.Apply("host-1", working, base.Add(2*time.Second))
	if working[0].WindowList[0].Status != tmux.SessionStatusRunning {
		t.Fatalf("expected changing screen to be running, got %q", working[0].WindowList[0].Status)
	}

	// Screen frozen past the running window: waiting again.
	settled := screenProbeSessions(screenProbe{hash: "h2", status: "running", width: 120})
	tracker.Apply("host-1", settled, base.Add(10*time.Second))
	if settled[0].WindowList[0].Status != tmux.SessionStatusWaiting {
		t.Fatalf("expected frozen screen to settle to waiting, got %q", settled[0].WindowList[0].Status)
	}
}

func TestTrackerIgnoresRewrapAfterResize(t *testing.T) {
	tracker := newPaneActivityTracker()
	base := time.Unix(1710000000, 0).UTC()
	tracker.Apply("host-1", screenProbeSessions(screenProbe{hash: "h1", status: "waiting", width: 120}), base)

	// Resize rewraps the grid: hash changed, but it is not real output.
	rewrapped := screenProbeSessions(screenProbe{hash: "h2", status: "running", width: 150})
	tracker.Apply("host-1", rewrapped, base.Add(2*time.Second))
	if rewrapped[0].WindowList[0].Status != tmux.SessionStatusWaiting {
		t.Fatalf("expected rewrap to stay waiting, got %q", rewrapped[0].WindowList[0].Status)
	}

	// Content changing again well after the resize is real output.
	working := screenProbeSessions(screenProbe{hash: "h3", status: "waiting", width: 150})
	tracker.Apply("host-1", working, base.Add(10*time.Second))
	if working[0].WindowList[0].Status != tmux.SessionStatusRunning {
		t.Fatalf("expected post-resize output to be running, got %q", working[0].WindowList[0].Status)
	}
}

func TestTrackerLeavesTitleVerdictAndDeadPanesAlone(t *testing.T) {
	tracker := newPaneActivityTracker()
	base := time.Unix(1710000000, 0).UTC()
	tracker.Apply("host-1", screenProbeSessions(screenProbe{hash: "h1", status: "running", title: "⠂ topic", width: 120}), base)

	// Claude's spinner title is authoritative even with a frozen screen.
	claude := screenProbeSessions(screenProbe{hash: "h1", status: "running", title: "⠂ topic", width: 120})
	tracker.Apply("host-1", claude, base.Add(2*time.Second))
	if claude[0].WindowList[0].Status != tmux.SessionStatusRunning {
		t.Fatalf("expected title verdict to win, got %q", claude[0].WindowList[0].Status)
	}

	// A dead pane keeps its exit status.
	dead := screenProbeSessions(screenProbe{hash: "h1", status: "failed", width: 120})
	tracker.Apply("host-1", dead, base.Add(4*time.Second))
	if dead[0].WindowList[0].Status != tmux.SessionStatusFailed {
		t.Fatalf("expected dead pane to stay failed, got %q", dead[0].WindowList[0].Status)
	}
}

func TestTrackerWithoutFingerprintKeepsStatus(t *testing.T) {
	tracker := newPaneActivityTracker()
	base := time.Unix(1710000000, 0).UTC()

	// Legacy output without scan lines: activity-based status passes through.
	legacy := screenProbeSessions(screenProbe{hash: "", status: "running", width: 0})
	tracker.Apply("host-1", legacy, base)
	if legacy[0].WindowList[0].Status != tmux.SessionStatusRunning {
		t.Fatalf("expected fingerprint-less window to keep status, got %q", legacy[0].WindowList[0].Status)
	}
}
