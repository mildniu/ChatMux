import { useMemo } from "react";
import { type SessionStatus, type TmuxSession, type TmuxWindow } from "./api";
import { unreadWindowKey, windowStableId } from "./unread-windows";

export type DisplayTmuxWindow = TmuxWindow & { unread: boolean };

export type DisplayTmuxSession = Omit<TmuxSession, "windowList"> & {
  displayStatus: SessionStatus;
  statusLabel: string;
  unreadWindows: number;
  windowList: DisplayTmuxWindow[];
};

/**
 * Decorate the backend sessions for display. Statuses pass through untouched —
 * the gateway is the single source of truth for what a pane is doing — and
 * each window gains its unread flag.
 */
export function useDisplaySessions(hostId: string, sessions: TmuxSession[], unreadKeys: ReadonlySet<string>) {
  return useMemo(
    () => sessions.map((session) => displaySession(hostId, session, unreadKeys)),
    [hostId, sessions, unreadKeys],
  );
}

export function displaySession(hostId: string, session: TmuxSession, unreadKeys: ReadonlySet<string>): DisplayTmuxSession {
  const windowList = session.windowList.map((window) => ({
    ...window,
    unread: unreadKeys.has(unreadWindowKey(hostId, session.name, windowStableId(window))),
  }));
  return {
    ...session,
    displayStatus: session.status,
    statusLabel: sessionStatusLabel(session.status, session.processName),
    unreadWindows: windowList.reduce((count, window) => count + (window.unread ? 1 : 0), 0),
    windowList,
  };
}

function sessionStatusLabel(status: SessionStatus, processName: string) {
  if (status === "running" && processName) {
    return `${processName} running`;
  }
  if (status === "waiting" && processName) {
    return `${processName} waiting`;
  }
  return status;
}
