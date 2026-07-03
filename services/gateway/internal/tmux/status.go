package tmux

import (
	"strconv"
	"strings"
	"time"
)

// paneRunningActivityWindow is how long a pane may stay quiet before a
// foreground program is considered to have finished its output. Busy TUIs and
// build tools emit output at least about once per second, so a few quiet
// seconds reliably mean "finished / awaiting input". Kept above the client's
// 2s poll interval so short gaps between output bursts do not flicker.
const paneRunningActivityWindow = 5 * time.Second

const (
	SessionStatusDone    = "done"
	SessionStatusFailed  = "failed"
	SessionStatusIdle    = "idle"
	SessionStatusRunning = "running"
	SessionStatusUnknown = "unknown"
	SessionStatusWaiting = "waiting"
)

// Mirrors SHELL_PROCESS_NAMES in apps/web/src/session-window-utils.ts.
var shellCommandNames = map[string]struct{}{
	"bash": {},
	"csh":  {},
	"dash": {},
	"fish": {},
	"ksh":  {},
	"sh":   {},
	"tcsh": {},
	"zsh":  {},
}

type sessionStatusInput struct {
	currentCommand string
	now            time.Time
	paneDead       bool
	paneDeadStatus string
	paneTitle      string
	updatedAt      time.Time
}

// sessionStatus classifies what the pane is actually doing:
//
//	done/failed — the pane's process exited (exit code decides which)
//	idle        — a shell prompt is in the foreground; a prompt is never running
//	running     — a foreground program is actively producing output
//	waiting     — a foreground program is alive but quiet: output finished,
//	              awaiting input
//
// The shell check must run before the title check: a terminal title outlives
// the program that set it, so after claude exits the shell may still carry a
// "✳ …" title.
//
// The pane title is the highest-signal source for agents: Claude Code sets a
// braille-spinner title prefix while working and an asterisk prefix while
// waiting for input. Its idle status line still repaints every ~10 seconds, so
// activity alone cannot tell working from idle. Programs that do not manage
// the title (codex, builds) fall back to the activity window; a silent
// long-running command is reported as waiting — tmux exposes no process state,
// only output activity.
func sessionStatus(input sessionStatusInput) string {
	if input.paneDead {
		return deadPaneStatus(input.paneDeadStatus)
	}
	command := normalizePaneCommand(input.currentCommand)
	if command == "" {
		return SessionStatusUnknown
	}
	if isShellCommand(command) {
		return SessionStatusIdle
	}
	if titleStatus := PaneTitleStatus(input.paneTitle); titleStatus != "" {
		return titleStatus
	}
	if sessionActivityIsRecent(input.updatedAt, input.now) {
		return SessionStatusRunning
	}
	return SessionStatusWaiting
}

func isShellCommand(command string) bool {
	_, ok := shellCommandNames[command]
	return ok
}

// PaneTitleStatus reads the working/waiting marker that Claude Code (2.x)
// writes as the first rune of the terminal title: a braille spinner cell
// (U+2801-U+28FF, excluding the blank cell) while working, "✳" (or "✻" in
// some builds) while waiting for input.
func PaneTitleStatus(title string) string {
	for _, r := range title {
		if r > 0x2800 && r <= 0x28FF {
			return SessionStatusRunning
		}
		if r == '✳' || r == '✻' {
			return SessionStatusWaiting
		}
		break
	}
	return ""
}

func sessionActivityIsRecent(updatedAt time.Time, now time.Time) bool {
	if updatedAt.IsZero() {
		return false
	}
	if now.IsZero() {
		now = time.Now()
	}
	return !now.Before(updatedAt) && now.Sub(updatedAt) <= paneRunningActivityWindow
}

func deadPaneStatus(status string) string {
	trimmed := strings.TrimSpace(status)
	if trimmed == "" {
		return SessionStatusUnknown
	}
	exitCode, err := strconv.Atoi(trimmed)
	if err != nil {
		return SessionStatusUnknown
	}
	if exitCode == 0 {
		return SessionStatusDone
	}
	return SessionStatusFailed
}

func normalizePaneCommand(command string) string {
	command = strings.ToLower(strings.TrimSpace(command))
	if index := strings.LastIndex(command, "/"); index >= 0 {
		command = command[index+1:]
	}
	return strings.TrimPrefix(command, "-")
}
