package tmux

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type Window struct {
	ID          string    `json:"id"`
	Index       int       `json:"index"`
	Name        string    `json:"name"`
	Active      bool      `json:"active"`
	UpdatedAt   time.Time `json:"updatedAt"`
	Status      string    `json:"status"`
	ProcessName string    `json:"processName"`
	AutoRename  bool      `json:"autoRename"`
	PaneTitle   string    `json:"paneTitle"`
	// Window dimensions and the visible-screen checksum feed the gateway's
	// screen-fingerprint activity tracking; they are not part of the API
	// payload.
	Width      int    `json:"-"`
	Height     int    `json:"-"`
	ScreenHash string `json:"-"`
}

func parseWindowLine(line string, now time.Time) (Window, string, error) {
	// pane_title is the last field and may itself contain tabs, so the split
	// is capped at the field count and the title keeps any embedded tabs.
	parts := strings.SplitN(line, "\t", 14)
	if len(parts) != 14 {
		return Window{}, "", fmt.Errorf("invalid tmux window line: %q", line)
	}
	index, err := strconv.Atoi(parts[3])
	if err != nil {
		return Window{}, "", fmt.Errorf("parse tmux window index: %w", err)
	}
	active, err := parseTmuxBool(parts[5], "window active")
	if err != nil {
		return Window{}, "", err
	}
	activity, err := strconv.ParseInt(parts[6], 10, 64)
	if err != nil {
		return Window{}, "", fmt.Errorf("parse tmux window activity: %w", err)
	}
	paneDead, err := parseTmuxBool(parts[8], "pane dead")
	if err != nil {
		return Window{}, "", err
	}
	autoRename, err := parseTmuxBool(parts[10], "automatic-rename")
	if err != nil {
		return Window{}, "", err
	}
	width, height, err := parseWindowDimensions(parts[11], parts[12])
	if err != nil {
		return Window{}, "", err
	}
	updatedAt := time.Unix(activity, 0).UTC()
	processName := normalizePaneCommand(parts[7])
	paneTitle := normalizePaneTitle(parts[13])
	status := sessionStatus(sessionStatusInput{
		currentCommand: parts[7],
		now:            now,
		paneDead:       paneDead,
		paneDeadStatus: parts[9],
		paneTitle:      paneTitle,
		updatedAt:      updatedAt,
	})
	return Window{
		ID: parts[2], Index: index, Name: normalizeWindowName(parts[4]),
		Active: active, UpdatedAt: updatedAt, ProcessName: processName, Status: status,
		AutoRename: autoRename, PaneTitle: paneTitle, Width: width, Height: height,
	}, parts[1], nil
}

func parseWindowDimensions(width string, height string) (int, int, error) {
	parsedWidth, err := strconv.Atoi(width)
	if err != nil {
		return 0, 0, fmt.Errorf("parse tmux window width: %w", err)
	}
	parsedHeight, err := strconv.Atoi(height)
	if err != nil {
		return 0, 0, fmt.Errorf("parse tmux window height: %w", err)
	}
	return parsedWidth, parsedHeight, nil
}

func applyParsedWindows(
	sessions []Session,
	pendingWindows map[string][]Window,
	sessionIndexes map[string]int,
) []Session {
	for sessionName, windows := range pendingWindows {
		index, ok := sessionIndexes[sessionName]
		if !ok {
			continue
		}
		sessions[index].WindowList = windows
		sessions[index].Windows = len(windows)
		status, processName := AggregateSessionStatus(windows)
		sessions[index].Status = status
		sessions[index].ProcessName = processName
	}
	return sessions
}

var sessionStatusPrecedence = []string{
	SessionStatusRunning,
	SessionStatusWaiting,
	SessionStatusFailed,
	SessionStatusDone,
	SessionStatusIdle,
	SessionStatusUnknown,
}

// AggregateSessionStatus rolls window statuses up to the session: any running
// window makes the session running, else any waiting window, and so on down
// the precedence list. The process name follows the status-determining window,
// so a session shows "claude running" even while its active pane is a shell in
// another window.
func AggregateSessionStatus(windows []Window) (string, string) {
	for _, status := range sessionStatusPrecedence {
		for _, window := range windows {
			if window.Status == status {
				return status, window.ProcessName
			}
		}
	}
	return SessionStatusUnknown, ""
}

func defaultWindowList(sessionName string, updatedAt time.Time, processName string, status string) []Window {
	return []Window{{
		ID: sessionName + ":0", Index: 0, Name: "0",
		Active: true, UpdatedAt: updatedAt, ProcessName: processName, Status: status,
		AutoRename: true,
	}}
}

func normalizeWindowName(name string) string {
	if utf8.RuneCountInString(name) <= maxWindowNameRunes {
		return name
	}
	return string([]rune(name)[:maxWindowNameRunes])
}

// normalizePaneTitle trims surrounding whitespace and caps the length so an
// unusually long terminal title cannot bloat the response or the UI.
func normalizePaneTitle(title string) string {
	trimmed := strings.TrimSpace(title)
	if utf8.RuneCountInString(trimmed) <= maxWindowNameRunes {
		return trimmed
	}
	return string([]rune(trimmed)[:maxWindowNameRunes])
}
