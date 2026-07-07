package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/chatmux/chatmux/services/gateway/internal/hoststore"
	"github.com/chatmux/chatmux/services/gateway/internal/tmux"
)

type tmuxListRequest struct {
	CredentialToken string `json:"credentialToken"`
}

type tmuxCreateRequest struct {
	CredentialToken string `json:"credentialToken"`
	Name            string `json:"name"`
}

const fallbackSSHSessionName = "ssh"

func (s *Server) handleListTmuxSessions(w http.ResponseWriter, r *http.Request) {
	hostID, ok := routeHostAction(r.URL.Path, "/tmux/sessions/list")
	if !ok {
		writeError(w, http.StatusNotFound, errors.New("route not found"))
		return
	}

	var input tmuxListRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	host, err := s.visibleHost(r, hostID)
	if err != nil {
		writeError(w, statusForHostAccessError(err), err)
		return
	}
	credential, err := s.sshCredentialForRequest(r, hostID, input.CredentialToken)
	if err != nil {
		writeError(w, statusForCredentialError(err), err)
		return
	}

	sessions, err := s.runMuxListOnHost(r, host, credential, listMuxCommands())
	if err != nil {
		if session, ok := fallbackSessionFromTmuxError(err); ok {
			writeJSON(w, http.StatusOK, []tmux.Session{s.sshFallback.Session(hostID, session.UpdatedAt)})
			return
		}
		writeError(w, http.StatusBadGateway, err)
		return
	}
	s.paneActivity.Apply(hostID, sessions, time.Now())
	sessions, err = s.applyVisibleSessionMetadata(r, host, sessions)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := s.logAudit(r.Context(), hoststore.LogAuditEventInput{Type: "tmux.sessions.listed", HostID: hostID, Message: "listed tmux sessions"}); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, sessions)
}

func (s *Server) handleCreateTmuxSession(w http.ResponseWriter, r *http.Request) {
	hostID, ok := routeHostAction(r.URL.Path, "/tmux/sessions")
	if !ok {
		writeError(w, http.StatusNotFound, errors.New("route not found"))
		return
	}

	var input tmuxCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	command, err := createSessionCommands(input.Name)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	host, err := s.visibleHost(r, hostID)
	if err != nil {
		writeError(w, statusForHostAccessError(err), err)
		return
	}
	credential, err := s.sshCredentialForRequest(r, hostID, input.CredentialToken)
	if err != nil {
		writeError(w, statusForCredentialError(err), err)
		return
	}
	sessions, err := s.runMuxListOnHost(r, host, credential, command)
	if err != nil {
		if session, ok := fallbackSessionFromTmuxError(err); ok {
			writeJSON(w, http.StatusCreated, s.sshFallback.Session(hostID, session.UpdatedAt))
			return
		}
		writeError(w, http.StatusBadGateway, err)
		return
	}
	session, err := findSessionByName(sessions, input.Name)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	metadata, err := s.hosts.SaveSessionMetadata(r.Context(), hoststore.SaveSessionMetadataInput{
		HostID: hostID, Owner: requestPrincipal(r).Name, SessionName: session.Name,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	session = applySessionMetadata(session, metadata, true)
	if err := s.logAudit(r.Context(), hoststore.LogAuditEventInput{Type: "tmux.session.created", HostID: hostID, SessionName: session.Name, Message: "created tmux session"}); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, session)
}

func findSessionByName(sessions []tmux.Session, name string) (tmux.Session, error) {
	for _, session := range sessions {
		if session.Name == name {
			return session, nil
		}
	}
	return tmux.Session{}, errors.New("created tmux session was not found")
}

func fallbackSessionFromTmuxError(err error) (tmux.Session, bool) {
	output, ok := commandErrorOutput(err)
	if !ok || (!tmux.Unavailable(output) && !tmux.PSMuxUnavailable(output)) {
		return tmux.Session{}, false
	}
	return tmux.Session{UpdatedAt: time.Now().UTC()}, true
}

func createSessionCommands(name string) (muxCommands, error) {
	tmuxCommand, err := tmux.CreateSessionCommand(name)
	if err != nil {
		return muxCommands{}, err
	}
	psmuxCommand, err := tmux.CreatePSMuxSessionCommand(name)
	if err != nil {
		return muxCommands{}, err
	}
	return muxCommands{tmux: tmuxCommand, psmux: psmuxCommand}, nil
}
