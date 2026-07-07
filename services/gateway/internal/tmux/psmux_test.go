package tmux

import (
	"encoding/base64"
	"encoding/binary"
	"strings"
	"testing"
	"unicode/utf16"
)

func TestParsePSMuxSessionsSetsMode(t *testing.T) {
	output := strings.Join([]string{
		"session\t$0\twin\t1\t0\t1783410495\tpowershell\t0\t0\t1783410495",
		"window\twin\t@1\t0\tpowershell\t1\t1783410495\tpowershell\t0\t0\t1\t120\t30\tDESKTOP",
	}, "\n")
	sessions, err := ParsePSMuxSessions(output)
	if err != nil {
		t.Fatalf("ParsePSMuxSessions failed: %v", err)
	}
	if len(sessions) != 1 || sessions[0].Mode != "psmux" {
		t.Fatalf("expected psmux session, got %#v", sessions)
	}
	if len(sessions[0].WindowList) != 1 || !sessions[0].WindowList[0].AutoRename {
		t.Fatalf("expected parsed psmux window, got %#v", sessions[0].WindowList)
	}
}

func TestListPSMuxSessionsCommandUsesEncodedPowerShell(t *testing.T) {
	script := decodePSMuxCommand(t, ListPSMuxSessionsCommand())
	if !strings.Contains(script, "Get-Command psmux") {
		t.Fatalf("expected psmux lookup, got %s", script)
	}
	if !strings.Contains(script, "$ProgressPreference = 'SilentlyContinue'") {
		t.Fatalf("expected progress suppression, got %s", script)
	}
	if !strings.Contains(script, "list-sessions -F") || !strings.Contains(script, "list-windows -a -F") {
		t.Fatalf("expected list commands, got %s", script)
	}
	if !strings.Contains(script, "#{?automatic-rename,1,0}") {
		t.Fatalf("expected normalized automatic rename format, got %s", script)
	}
}

func TestAttachPSMuxTargetCommandTargetsWindow(t *testing.T) {
	windowIndex := 1
	command, err := AttachPSMuxTargetCommand(Target{SessionName: "deploy", WindowIndex: &windowIndex})
	if err != nil {
		t.Fatalf("AttachPSMuxTargetCommand failed: %v", err)
	}
	script := decodePSMuxCommand(t, command)
	if !strings.Contains(script, "set-option -gq history-limit") {
		t.Fatalf("expected history option, got %s", script)
	}
	if !strings.Contains(script, "attach-session -t '=deploy:1'") {
		t.Fatalf("expected target attach, got %s", script)
	}
}

func TestPSMuxSessionCommandsUseBarePureSessionTargets(t *testing.T) {
	killCommand, err := KillPSMuxSessionCommand("deploy")
	if err != nil {
		t.Fatalf("KillPSMuxSessionCommand failed: %v", err)
	}
	killScript := decodePSMuxCommand(t, killCommand)
	if !strings.Contains(killScript, "kill-session -t 'deploy'") || strings.Contains(killScript, "kill-session -t '=deploy'") {
		t.Fatalf("expected bare kill-session target, got %s", killScript)
	}

	renameCommand, err := RenamePSMuxSessionCommand("deploy", "deploy2")
	if err != nil {
		t.Fatalf("RenamePSMuxSessionCommand failed: %v", err)
	}
	renameScript := decodePSMuxCommand(t, renameCommand)
	if !strings.Contains(renameScript, "rename-session -t 'deploy' 'deploy2'") || strings.Contains(renameScript, "rename-session -t '=deploy'") {
		t.Fatalf("expected bare rename-session target, got %s", renameScript)
	}
}

func decodePSMuxCommand(t *testing.T, command string) string {
	t.Helper()
	const marker = "-EncodedCommand "
	index := strings.Index(command, marker)
	if index < 0 {
		t.Fatalf("expected encoded PowerShell command, got %q", command)
	}
	payload, err := base64.StdEncoding.DecodeString(strings.TrimSpace(command[index+len(marker):]))
	if err != nil {
		t.Fatalf("decode command: %v", err)
	}
	if len(payload)%2 != 0 {
		t.Fatalf("expected UTF-16LE payload, got %d bytes", len(payload))
	}
	encoded := make([]uint16, len(payload)/2)
	for index := range encoded {
		encoded[index] = binary.LittleEndian.Uint16(payload[index*2:])
	}
	return string(utf16.Decode(encoded))
}
