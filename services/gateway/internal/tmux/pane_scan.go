package tmux

import "strings"

// Agents that run behind a generic runtime: pane_current_command reports the
// interpreter (codex shows as "node"), but a process on the pane's tty names
// the agent binary.
var agentProcessNames = map[string]struct{}{
	"codex": {},
}

// paneScan collects the auxiliary lines emitted by rawPaneScanCommand: a
// screen-content checksum per active non-shell pane and a global tty-to-args
// process dump. These lines are best-effort (ps or capture-pane may fail on
// exotic hosts), so malformed ones are skipped rather than failing the parse.
type paneScan struct {
	agents  map[string]string // tty (e.g. "pts/5") -> agent process name
	screens map[string]string // window ID -> screen content checksum
	ttys    map[string]string // window ID -> tty of the active pane
}

func newPaneScan() *paneScan {
	return &paneScan{agents: map[string]string{}, screens: map[string]string{}, ttys: map[string]string{}}
}

func (s *paneScan) consumeLine(line string) bool {
	if rest, ok := strings.CutPrefix(line, listScreenPrefix); ok {
		s.consumeScreenLine(rest)
		return true
	}
	if rest, ok := strings.CutPrefix(line, listProcessPrefix); ok {
		s.consumeProcessLine(rest)
		return true
	}
	return false
}

func (s *paneScan) consumeScreenLine(rest string) {
	parts := strings.SplitN(rest, "\t", 3)
	if len(parts) != 3 || parts[0] == "" || parts[2] == "" {
		return
	}
	s.screens[parts[0]] = parts[2]
	s.ttys[parts[0]] = strings.TrimPrefix(parts[1], "/dev/")
}

func (s *paneScan) consumeProcessLine(rest string) {
	tty, args, found := strings.Cut(rest, "\t")
	if !found || tty == "" || tty == "?" {
		return
	}
	if agent := agentFromProcessArgs(args); agent != "" {
		s.agents[tty] = agent
	}
}

func (s *paneScan) applyToWindows(pendingWindows map[string][]Window) {
	if len(s.screens) == 0 {
		return
	}
	for sessionName := range pendingWindows {
		windows := pendingWindows[sessionName]
		for index := range windows {
			window := &windows[index]
			window.ScreenHash = s.screens[window.ID]
			if agent, ok := s.agents[s.ttys[window.ID]]; ok {
				window.ProcessName = agent
			}
		}
	}
}

// agentFromProcessArgs reports the agent whose binary name appears as a
// path component of the command line, e.g. "node /…/bin/codex" -> "codex".
func agentFromProcessArgs(args string) string {
	for _, token := range strings.Fields(args) {
		name := token
		if index := strings.LastIndex(name, "/"); index >= 0 {
			name = name[index+1:]
		}
		if _, ok := agentProcessNames[strings.ToLower(name)]; ok {
			return strings.ToLower(name)
		}
	}
	return ""
}
