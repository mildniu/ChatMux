import { useCallback, useEffect, useRef } from "react";
import { type TmuxSession } from "./api";
import { listTmuxSessions } from "./tmux-api";
import { useDisplaySessions } from "./session-state-machine";
import { useSessionNotifications } from "./useSessionNotifications";
import { useSessionStatusPolling } from "./useSessionStatusPolling";
import { useUnreadDocumentTitle, useUnreadWindows, windowIsViewed, type ViewedWindow } from "./useUnreadWindows";
import { diffSessionsSnapshot, type SessionsSnapshot, type WindowFinishedEvent } from "./window-transitions";

type SessionWorkspaceStateOptions = {
  hostId: string;
  hostName: string;
  selectedSessionName: string;
  selectedWindowIndex: number | null;
  sessions: TmuxSession[];
  sshReady: boolean;
  terminalVisible: boolean;
  getCredentialToken: () => Promise<string>;
  onError: (message: string) => void;
  onOpenWindow: (sessionName: string, windowIndex: number) => void;
  onSessionsChange: (sessions: TmuxSession[]) => void;
};

export function useSessionWorkspaceState(options: SessionWorkspaceStateOptions) {
  const refreshSessions = useCallback(async () => {
    if (!options.hostId || !options.sshReady) {
      return [];
    }
    const credentialToken = await options.getCredentialToken();
    return listTmuxSessions(options.hostId, credentialToken);
  }, [options.getCredentialToken, options.hostId, options.sshReady]);

  const viewed: ViewedWindow = {
    hostId: options.hostId,
    sessionName: options.selectedSessionName,
    terminalVisible: options.terminalVisible,
    windowIndex: options.selectedWindowIndex,
  };
  const unread = useUnreadWindows({ hostId: options.hostId, sessions: options.sessions, viewed });
  const notifications = useSessionNotifications({
    hostId: options.hostId,
    hostName: options.hostName,
    onError: options.onError,
    onOpenWindow: options.onOpenWindow,
    sshReady: options.sshReady,
  });
  useWindowFinishedEvents({
    hostId: options.hostId,
    sessions: options.sessions,
    viewed,
    onEvents: (events) => {
      unread.markUnread(events);
      notifications.notifyFinished(events);
    },
  });
  const displaySessions = useDisplaySessions(options.hostId, options.sessions, unread.unreadKeys);
  useUnreadDocumentTitle(unread.unreadCount);
  useSessionStatusPolling({
    hostId: options.hostId,
    onError: options.onError,
    onRefreshError: notifications.markRefreshError,
    onRefreshSuccess: notifications.markRefreshSuccess,
    onSessionsChange: options.onSessionsChange,
    refreshSessions,
    sshReady: options.sshReady,
  });
  return { displaySessions, notifications };
}

/**
 * Diff each poll result against the previous snapshot and surface finish
 * events for windows the user is not currently looking at.
 */
function useWindowFinishedEvents(args: {
  hostId: string;
  sessions: TmuxSession[];
  viewed: ViewedWindow;
  onEvents: (events: WindowFinishedEvent[]) => void;
}) {
  const snapshotRef = useRef<SessionsSnapshot | null>(null);
  useEffect(() => {
    const now = Date.now();
    const { events, snapshot } = diffSessionsSnapshot(args.hostId, args.sessions, snapshotRef.current, now);
    snapshotRef.current = snapshot;
    const unseen = events.filter((event) => !windowIsViewed(args.viewed, event.sessionName, event.windowIndex));
    if (unseen.length > 0) {
      args.onEvents(unseen);
    }
    // Only new poll data can produce transitions; viewed/onEvents are read
    // from the render that delivered the data.
  }, [args.hostId, args.sessions]);
}
