package api

import (
	"sync"
	"time"

	"github.com/chatmux/chatmux/services/gateway/internal/tmux"
)

// A tmux client attaching, detaching, or resizing makes full-screen TUIs
// repaint, and that repaint bumps #{window_activity} exactly like real output:
// merely opening a quiet codex window in the browser would flip it to
// "running" for the activity window. The repaint always follows a window size
// change, so after a size change is observed, activity-based "running" is
// ignored for a few seconds. A title-based "running" (claude's spinner) is
// real work and is never suppressed.
const resizeSuppressionWindow = 8 * time.Second

type windowResizeState struct {
	width         int
	height        int
	suppressUntil time.Time
}

type resizeSuppressor struct {
	mu    sync.Mutex
	hosts map[string]map[string]windowResizeState
}

func newResizeSuppressor() *resizeSuppressor {
	return &resizeSuppressor{hosts: map[string]map[string]windowResizeState{}}
}

// Apply rewrites resize-induced "running" window statuses to "waiting" in
// place and re-aggregates the affected sessions. It keeps the last seen
// dimensions per window; windows that disappeared are forgotten.
func (s *resizeSuppressor) Apply(hostID string, sessions []tmux.Session, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	previous := s.hosts[hostID]
	next := map[string]windowResizeState{}
	for sessionIndex := range sessions {
		session := &sessions[sessionIndex]
		suppressed := false
		for windowIndex := range session.WindowList {
			window := &session.WindowList[windowIndex]
			key := session.Name + "\x00" + window.ID
			state := nextResizeState(previous[key], window, now)
			next[key] = state
			if suppressResizeRepaint(window, state, now) {
				window.Status = tmux.SessionStatusWaiting
				suppressed = true
			}
		}
		if suppressed {
			status, processName := tmux.AggregateSessionStatus(session.WindowList)
			session.Status = status
			session.ProcessName = processName
		}
	}
	s.hosts[hostID] = next
}

func nextResizeState(previous windowResizeState, window *tmux.Window, now time.Time) windowResizeState {
	state := windowResizeState{
		width:         window.Width,
		height:        window.Height,
		suppressUntil: previous.suppressUntil,
	}
	// A window never seen before (or from legacy output without dimensions)
	// has no baseline to compare against.
	if previous.width == 0 && previous.height == 0 {
		return state
	}
	if previous.width != window.Width || previous.height != window.Height {
		state.suppressUntil = now.Add(resizeSuppressionWindow)
	}
	return state
}

func suppressResizeRepaint(window *tmux.Window, state windowResizeState, now time.Time) bool {
	return window.Status == tmux.SessionStatusRunning &&
		now.Before(state.suppressUntil) &&
		tmux.PaneTitleStatus(window.PaneTitle) != tmux.SessionStatusRunning
}
