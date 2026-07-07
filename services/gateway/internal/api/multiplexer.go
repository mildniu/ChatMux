package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/chatmux/chatmux/services/gateway/internal/hoststore"
	"github.com/chatmux/chatmux/services/gateway/internal/sshclient"
	"github.com/chatmux/chatmux/services/gateway/internal/tmux"
)

type muxCommands struct {
	tmux  string
	psmux string
}

func listMuxCommands() muxCommands {
	return muxCommands{tmux: tmux.ListSessionsCommand(), psmux: tmux.ListPSMuxSessionsCommand()}
}

func (s *Server) runMuxListCommand(
	r *http.Request,
	hostID string,
	credential sshclient.Credential,
	commands muxCommands,
) ([]tmux.Session, error) {
	host, err := s.visibleHost(r, hostID)
	if err != nil {
		return nil, err
	}
	sessions, err := s.runMuxListOnHost(r, host, credential, commands)
	if err != nil {
		return nil, err
	}
	s.paneActivity.Apply(hostID, sessions, time.Now())
	return s.applyVisibleSessionMetadata(r, host, sessions)
}

func (s *Server) runMuxListOnHost(
	r *http.Request,
	host hoststore.Host,
	credential sshclient.Credential,
	commands muxCommands,
) ([]tmux.Session, error) {
	output, err := s.ssh.Run(r.Context(), hostToSSHConfig(host), credential, commands.tmux)
	if err == nil {
		return tmux.ParseSessions(string(output))
	}
	if !shouldTryPSMux(err) {
		return nil, err
	}
	output, err = s.ssh.Run(r.Context(), hostToSSHConfig(host), credential, commands.psmux)
	if err != nil {
		return nil, err
	}
	return tmux.ParsePSMuxSessions(string(output))
}

func (s *Server) runMuxOutputCommand(
	r *http.Request,
	host hoststore.Host,
	credential sshclient.Credential,
	commands muxCommands,
) ([]byte, error) {
	output, err := s.ssh.Run(r.Context(), hostToSSHConfig(host), credential, commands.tmux)
	if err == nil {
		return output, nil
	}
	if !shouldTryPSMux(err) {
		return nil, err
	}
	return s.ssh.Run(r.Context(), hostToSSHConfig(host), credential, commands.psmux)
}

func shouldTryPSMux(err error) bool {
	output, ok := commandErrorOutput(err)
	return ok && tmux.UnsupportedLoginShell(output)
}

func commandErrorOutput(err error) (string, bool) {
	var commandError sshclient.CommandError
	if !errors.As(err, &commandError) {
		return "", false
	}
	return commandError.Output, true
}
