package api

import (
	"testing"
	"time"

	"github.com/chatmux/chatmux/services/gateway/internal/tmux"
)

func resizeProbeSessions(status string, width int, title string) []tmux.Session {
	windows := []tmux.Window{{
		ID: "@1", Index: 0, Status: status, ProcessName: "node",
		PaneTitle: title, Width: width, Height: 40,
	}}
	sessionStatus, processName := tmux.AggregateSessionStatus(windows)
	return []tmux.Session{{
		Name: "dev", Status: sessionStatus, ProcessName: processName, WindowList: windows,
	}}
}

func TestResizeSuppressorRewritesRepaintRunning(t *testing.T) {
	suppressor := newResizeSuppressor()
	base := time.Unix(1710000000, 0).UTC()

	// Baseline poll: quiet window at 120 columns.
	first := resizeProbeSessions(tmux.SessionStatusWaiting, 120, "binjie09")
	suppressor.Apply("host-1", first, base)

	// The browser attached and resized the window; the repaint made tmux
	// report recent activity, i.e. "running".
	flapped := resizeProbeSessions(tmux.SessionStatusRunning, 150, "binjie09")
	suppressor.Apply("host-1", flapped, base.Add(2*time.Second))
	if flapped[0].Status != tmux.SessionStatusWaiting || flapped[0].WindowList[0].Status != tmux.SessionStatusWaiting {
		t.Fatalf("expected resize repaint to stay waiting, got session=%q window=%q",
			flapped[0].Status, flapped[0].WindowList[0].Status)
	}

	// Still "running" inside the suppression window: stays suppressed.
	still := resizeProbeSessions(tmux.SessionStatusRunning, 150, "binjie09")
	suppressor.Apply("host-1", still, base.Add(6*time.Second))
	if still[0].WindowList[0].Status != tmux.SessionStatusWaiting {
		t.Fatalf("expected continued suppression, got %q", still[0].WindowList[0].Status)
	}

	// Real work continuing past the suppression window shows as running.
	working := resizeProbeSessions(tmux.SessionStatusRunning, 150, "binjie09")
	suppressor.Apply("host-1", working, base.Add(12*time.Second))
	if working[0].Status != tmux.SessionStatusRunning {
		t.Fatalf("expected running after suppression lapses, got %q", working[0].Status)
	}
}

func TestResizeSuppressorKeepsTitleRunning(t *testing.T) {
	suppressor := newResizeSuppressor()
	base := time.Unix(1710000000, 0).UTC()
	suppressor.Apply("host-1", resizeProbeSessions(tmux.SessionStatusWaiting, 120, "✳ topic"), base)

	// A claude spinner title is real work even right after a resize.
	working := resizeProbeSessions(tmux.SessionStatusRunning, 150, "⠂ topic")
	suppressor.Apply("host-1", working, base.Add(2*time.Second))
	if working[0].WindowList[0].Status != tmux.SessionStatusRunning {
		t.Fatalf("expected spinner title to stay running, got %q", working[0].WindowList[0].Status)
	}
}

func TestResizeSuppressorIgnoresStableSize(t *testing.T) {
	suppressor := newResizeSuppressor()
	base := time.Unix(1710000000, 0).UTC()
	suppressor.Apply("host-1", resizeProbeSessions(tmux.SessionStatusWaiting, 120, "binjie09"), base)

	// Same dimensions: a running status is genuine output.
	working := resizeProbeSessions(tmux.SessionStatusRunning, 120, "binjie09")
	suppressor.Apply("host-1", working, base.Add(2*time.Second))
	if working[0].WindowList[0].Status != tmux.SessionStatusRunning {
		t.Fatalf("expected stable-size running to pass through, got %q", working[0].WindowList[0].Status)
	}
}

func TestResizeSuppressorFirstSightingHasNoBaseline(t *testing.T) {
	suppressor := newResizeSuppressor()
	base := time.Unix(1710000000, 0).UTC()

	// First observation ever (e.g. gateway restart): nothing to compare, no
	// suppression even though the status is running.
	working := resizeProbeSessions(tmux.SessionStatusRunning, 150, "binjie09")
	suppressor.Apply("host-1", working, base)
	if working[0].WindowList[0].Status != tmux.SessionStatusRunning {
		t.Fatalf("expected first sighting to pass through, got %q", working[0].WindowList[0].Status)
	}
}
