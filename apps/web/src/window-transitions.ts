import { type SessionStatus, type TmuxSession, type TmuxWindow } from "./api";
import { windowDisplayLabel } from "./session-window-utils";
import { isSSHFallbackSession } from "./tmux-fallback";
import { windowStableId } from "./unread-windows";

/** A window's foreground program stopped producing output or exited. */
export type WindowFinishedEvent = {
  hostId: string;
  nextStatus: SessionStatus;
  processName: string;
  sessionName: string;
  sessionTitle: string;
  windowId: string;
  windowIndex: number;
  windowLabel: string;
};

type WindowSnapshot = {
  processName: string;
  runningSince: number;
  status: SessionStatus;
};

export type SessionsSnapshot = {
  hostId: string;
  windows: Map<string, WindowSnapshot>;
};

// Typing into an idle TUI echoes output and looks "running" for a moment;
// only a sustained run counts as work whose end is worth announcing.
const minRunningMsBeforeFinish = 3_000;

const finishedStatuses: ReadonlySet<SessionStatus> = new Set(["done", "failed", "idle", "waiting"]);

/**
 * Compare the freshly polled sessions against the previous snapshot and
 * return the finish events plus the next snapshot. A null previous snapshot
 * (first poll, page reload) or a host switch yields no events.
 */
export function diffSessionsSnapshot(
  hostId: string,
  sessions: TmuxSession[],
  previous: SessionsSnapshot | null,
  now: number,
): { events: WindowFinishedEvent[]; snapshot: SessionsSnapshot } {
  const usablePrevious = previous && previous.hostId === hostId ? previous : null;
  const snapshot: SessionsSnapshot = { hostId, windows: new Map() };
  const events: WindowFinishedEvent[] = [];
  for (const session of sessions) {
    if (isSSHFallbackSession(session)) {
      continue;
    }
    for (const window of session.windowList) {
      const key = `${session.name}:${windowStableId(window)}`;
      const previousWindow = usablePrevious?.windows.get(key);
      snapshot.windows.set(key, nextWindowSnapshot(window, previousWindow, now));
      const event = windowFinishedEvent(hostId, session, window, previousWindow, now);
      if (event) {
        events.push(event);
      }
    }
  }
  return { events, snapshot };
}

function nextWindowSnapshot(window: TmuxWindow, previous: WindowSnapshot | undefined, now: number): WindowSnapshot {
  const stillRunning = window.status === "running" && previous?.status === "running";
  return {
    processName: window.processName,
    runningSince: window.status === "running" ? (stillRunning ? previous.runningSince : now) : 0,
    status: window.status,
  };
}

function windowFinishedEvent(
  hostId: string,
  session: TmuxSession,
  window: TmuxWindow,
  previous: WindowSnapshot | undefined,
  now: number,
): WindowFinishedEvent | null {
  if (previous?.status !== "running" || !finishedStatuses.has(window.status)) {
    return null;
  }
  if (now - previous.runningSince < minRunningMsBeforeFinish) {
    return null;
  }
  return {
    hostId,
    nextStatus: window.status,
    // The previous snapshot names the program that was running: when claude
    // exits back to zsh the event still belongs to claude.
    processName: previous.processName || window.processName,
    sessionName: session.name,
    sessionTitle: session.title || session.name,
    windowId: windowStableId(window),
    windowIndex: window.index,
    windowLabel: windowDisplayLabel(window),
  };
}
