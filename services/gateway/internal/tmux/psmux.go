package tmux

import (
	"encoding/base64"
	"encoding/binary"
	"strconv"
	"strings"
	"unicode/utf16"
)

const missingPSMuxMessage = "psmux not found in PATH, CHATMUX_PSMUX_BIN, or WinGet Links"

func ListPSMuxSessionsCommand() string {
	return windowsPowerShellCommand(psmuxListSessionsScript())
}

func CreatePSMuxSessionCommand(name string) (string, error) {
	if err := ValidateSessionName(name); err != nil {
		return "", err
	}
	lines := []string{"Invoke-PSMux new-session -d -s " + psQuote(name)}
	lines = append(lines, psmuxOptionLines(name)...)
	lines = append(lines, psmuxListLines()...)
	return windowsPowerShellCommand(psmuxScript(lines...)), nil
}

func AttachPSMuxTargetCommand(target Target) (string, error) {
	if err := ValidateTarget(target); err != nil {
		return "", err
	}
	lines := psmuxOptionLines(target.SessionName)
	lines = append(lines, "& $PSMuxBin attach-session -t "+psQuote(formatTarget(target)))
	lines = append(lines, "exit $LASTEXITCODE")
	return windowsPowerShellCommand(psmuxScript(lines...)), nil
}

func KillPSMuxSessionCommand(name string) (string, error) {
	if err := ValidateSessionName(name); err != nil {
		return "", err
	}
	lines := []string{"Invoke-PSMux kill-session -t " + psQuote(formatSessionOptionTarget(name))}
	lines = append(lines, psmuxListLines()...)
	return windowsPowerShellCommand(psmuxScript(lines...)), nil
}

func CapturePSMuxTargetPaneCommand(target Target, options CapturePaneOptions) (string, error) {
	if err := ValidateTarget(target); err != nil {
		return "", err
	}
	args := []string{"capture-pane", "-p"}
	if options.PreserveANSI {
		args = append(args, "-e", "-C")
	}
	args = append(args, "-t", formatTarget(target), "-S", "-"+strconv.Itoa(normalizeCapturePaneLines(options.Lines)))
	return windowsPowerShellCommand(psmuxScript("Invoke-PSMux " + psCommandArgs(args))), nil
}

func CreatePSMuxWindowCommand(sessionName string, windowName string, sourceWindowIndex *int) (string, error) {
	if err := ValidateSessionName(sessionName); err != nil {
		return "", err
	}
	if err := ValidateWindowName(windowName); err != nil {
		return "", err
	}
	sourceTarget := Target{SessionName: sessionName, WindowIndex: sourceWindowIndex}
	if err := ValidateTarget(sourceTarget); err != nil {
		return "", err
	}
	lines := []string{
		"$CurrentPath = & $PSMuxBin display-message -p -t " + psQuote(formatTarget(sourceTarget)) + " '#{pane_current_path}'",
		"if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }",
		"Invoke-PSMux new-window -d -t " + psQuote(formatNewWindowTarget(sessionName)) + " -c $CurrentPath -n " + psQuote(windowName),
	}
	lines = append(lines, psmuxListLines()...)
	return windowsPowerShellCommand(psmuxScript(lines...)), nil
}

func CurrentPSMuxPathCommand(target Target) (string, error) {
	if err := ValidateTarget(target); err != nil {
		return "", err
	}
	line := "Invoke-PSMux display-message -p -t " + psQuote(formatTarget(target)) + " '#{pane_current_path}'"
	return windowsPowerShellCommand(psmuxScript(line)), nil
}

func KillPSMuxWindowCommand(target Target) (string, error) {
	if err := ValidateTarget(target); err != nil {
		return "", err
	}
	lines := []string{"Invoke-PSMux kill-window -t " + psQuote(formatTarget(target))}
	lines = append(lines, psmuxListLines()...)
	return windowsPowerShellCommand(psmuxScript(lines...)), nil
}

func RenamePSMuxWindowCommand(target Target, name string) (string, error) {
	if err := ValidateTarget(target); err != nil {
		return "", err
	}
	if err := ValidateWindowName(name); err != nil {
		return "", err
	}
	lines := []string{"Invoke-PSMux rename-window -t " + psQuote(formatTarget(target)) + " " + psQuote(name)}
	lines = append(lines, psmuxListLines()...)
	return windowsPowerShellCommand(psmuxScript(lines...)), nil
}

func MovePSMuxWindowsCommand(sessionName string, swaps [][]int) (string, error) {
	if err := ValidateSessionName(sessionName); err != nil {
		return "", err
	}
	if len(swaps) == 0 {
		return ListPSMuxSessionsCommand(), nil
	}
	lines := make([]string, 0, len(swaps)+6)
	for _, swap := range swaps {
		line, err := psmuxSwapLine(sessionName, swap)
		if err != nil {
			return "", err
		}
		lines = append(lines, line)
	}
	lines = append(lines, psmuxListLines()...)
	return windowsPowerShellCommand(psmuxScript(lines...)), nil
}

func RenamePSMuxSessionCommand(sessionName string, newName string) (string, error) {
	if err := ValidateSessionName(sessionName); err != nil {
		return "", err
	}
	if err := ValidateSessionName(newName); err != nil {
		return "", err
	}
	lines := []string{"Invoke-PSMux rename-session -t " + psQuote(formatSessionOptionTarget(sessionName)) + " " + psQuote(newName)}
	lines = append(lines, psmuxListLines()...)
	return windowsPowerShellCommand(psmuxScript(lines...)), nil
}

func ParsePSMuxSessions(output string) ([]Session, error) {
	sessions, err := ParseSessions(output)
	if err != nil {
		return nil, err
	}
	for index := range sessions {
		sessions[index].Mode = "psmux"
	}
	return sessions, nil
}

func PSMuxUnavailable(output string) bool {
	return strings.Contains(output, missingPSMuxMessage)
}

func psmuxListSessionsScript() string {
	return psmuxScript(psmuxListLines()...)
}

func psmuxScript(lines ...string) string {
	return strings.Join(append(psmuxPreludeLines(), lines...), "\n")
}

func psmuxPreludeLines() []string {
	return []string{
		"$ErrorActionPreference = 'Stop'",
		"$ProgressPreference = 'SilentlyContinue'",
		"$PSMuxBin = $env:CHATMUX_PSMUX_BIN",
		"if (-not $PSMuxBin) { $cmd = Get-Command psmux -ErrorAction SilentlyContinue; if ($cmd) { $PSMuxBin = $cmd.Source } }",
		"if (-not $PSMuxBin) { $cmd = Get-Command tmux -ErrorAction SilentlyContinue; if ($cmd) { $PSMuxBin = $cmd.Source } }",
		"if (-not $PSMuxBin) { [Console]::Error.WriteLine(" + psQuote(missingPSMuxMessage) + "); exit 127 }",
		"$HistoryLimit = $env:CHATMUX_TMUX_HISTORY_LIMIT",
		"if (-not $HistoryLimit) { $HistoryLimit = '" + strconv.Itoa(tmuxDefaultHistoryLimit) + "' }",
		"function Invoke-PSMux { & $PSMuxBin @args; if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE } }",
	}
}

func psmuxOptionLines(sessionName string) []string {
	return []string{
		"Invoke-PSMux set-option -gq history-limit $HistoryLimit",
		"Invoke-PSMux set-option -gq mouse on",
		"Invoke-PSMux set-option -t " + psQuote(formatSessionOptionTarget(sessionName)) + " -q history-limit $HistoryLimit",
		"Invoke-PSMux set-option -t " + psQuote(formatSessionOptionTarget(sessionName)) + " -q mouse on",
	}
}

func psmuxListLines() []string {
	return []string{
		"$Sessions = & $PSMuxBin list-sessions -F " + psQuote(listSessionFormat),
		"if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }",
		"$Windows = & $PSMuxBin list-windows -a -F " + psQuote(listWindowFormat),
		"if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }",
		"Write-Output ('__chatmux_now\t' + [DateTimeOffset]::UtcNow.ToUnixTimeSeconds())",
		"$Sessions | ForEach-Object { $_ }",
		"$Windows | ForEach-Object { $_ }",
	}
}

func psmuxSwapLine(sessionName string, swap []int) (string, error) {
	if len(swap) != 2 || swap[0] < 0 || swap[1] < 0 {
		return "", ErrInvalidWindowTarget
	}
	from := formatTarget(Target{SessionName: sessionName, WindowIndex: &swap[0]})
	to := formatTarget(Target{SessionName: sessionName, WindowIndex: &swap[1]})
	return "Invoke-PSMux swap-window -s " + psQuote(from) + " -t " + psQuote(to), nil
}

func psCommandArgs(args []string) string {
	quoted := make([]string, len(args))
	for index, arg := range args {
		quoted[index] = psQuote(arg)
	}
	return strings.Join(quoted, " ")
}

func psQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func windowsPowerShellCommand(script string) string {
	encoded := utf16.Encode([]rune(script))
	payload := make([]byte, len(encoded)*2)
	for index, value := range encoded {
		binary.LittleEndian.PutUint16(payload[index*2:], value)
	}
	return "powershell.exe -NoProfile -ExecutionPolicy Bypass -EncodedCommand " +
		base64.StdEncoding.EncodeToString(payload)
}
