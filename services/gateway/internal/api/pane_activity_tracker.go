package api

import (
	"sync"
	"time"

	"github.com/chatmux/chatmux/services/gateway/internal/tmux"
)

// #{window_activity} bumps on ANY pane output, including the repaints that a
// full-screen TUI performs when a client attaches, detaches, resizes, or
// changes focus — so merely opening a quiet codex window flipped it to
// "running". The screen fingerprint (a checksum of the visible pane content,
// collected by the same remote command) is immune to that: a repaint draws the
// identical screen and the checksum does not move, while a genuinely working
// program (streaming output, a ticking spinner or timer) changes it on every
// poll. For panes with a fingerprint, the tracker replaces the
// activity-timestamp guess entirely.
const screenChangeRunningWindow = 5 * time.Second

// A size change rewraps the grid and changes the checksum without any real
// output; checksum churn right after a resize is ignored.
const resizeRewrapGrace = 4 * time.Second

type windowScreenState struct {
	changedAt  time.Time
	height     int
	resizedAt  time.Time
	screenHash string
	width      int
}

type paneActivityTracker struct {
	mu    sync.Mutex
	hosts map[string]map[string]windowScreenState
}

func newPaneActivityTracker() *paneActivityTracker {
	return &paneActivityTracker{hosts: map[string]map[string]windowScreenState{}}
}

// Apply rewrites window statuses from the screen fingerprint in place and
// re-aggregates the affected sessions. It keeps per-window state across polls;
// windows that disappeared are forgotten.
func (t *paneActivityTracker) Apply(hostID string, sessions []tmux.Session, now time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	previous := t.hosts[hostID]
	next := map[string]windowScreenState{}
	for sessionIndex := range sessions {
		session := &sessions[sessionIndex]
		rewritten := false
		for windowIndex := range session.WindowList {
			window := &session.WindowList[windowIndex]
			if window.ScreenHash == "" {
				continue
			}
			key := session.Name + "\x00" + window.ID
			state, seen := previous[key]
			state = advanceScreenState(state, seen, window, now)
			next[key] = state
			if rewriteScreenStatus(window, state, now) {
				rewritten = true
			}
		}
		if rewritten {
			status, processName := tmux.AggregateSessionStatus(session.WindowList)
			session.Status = status
			session.ProcessName = processName
		}
	}
	t.hosts[hostID] = next
}

func advanceScreenState(state windowScreenState, seen bool, window *tmux.Window, now time.Time) windowScreenState {
	if !seen {
		// First sighting: no baseline, so nothing counts as a change yet. A
		// genuinely working pane flips to running on the next poll.
		return windowScreenState{width: window.Width, height: window.Height, screenHash: window.ScreenHash}
	}
	if state.width != window.Width || state.height != window.Height {
		state.resizedAt = now
		state.width = window.Width
		state.height = window.Height
	}
	if window.ScreenHash != state.screenHash {
		state.screenHash = window.ScreenHash
		if state.resizedAt.IsZero() || now.Sub(state.resizedAt) > resizeRewrapGrace {
			state.changedAt = now
		}
	}
	return state
}

// rewriteScreenStatus replaces an activity-derived running/waiting with the
// fingerprint verdict. Dead panes (done/failed) and unknown keep their status,
// and a title-based verdict (claude's spinner/asterisk) stays authoritative.
// Returns whether the status was changed.
func rewriteScreenStatus(window *tmux.Window, state windowScreenState, now time.Time) bool {
	if window.Status != tmux.SessionStatusRunning && window.Status != tmux.SessionStatusWaiting {
		return false
	}
	if tmux.PaneTitleStatus(window.PaneTitle) != "" {
		return false
	}
	status := tmux.SessionStatusWaiting
	if !state.changedAt.IsZero() && now.Sub(state.changedAt) <= screenChangeRunningWindow {
		status = tmux.SessionStatusRunning
	}
	if window.Status == status {
		return false
	}
	window.Status = status
	return true
}
