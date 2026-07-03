import { type SessionStatus, type TmuxSession, type TmuxWindow } from "./api";

export type UnreadWindowRecord = {
  processName: string;
  since: number;
  status: SessionStatus;
};

export type UnreadWindows = Record<string, UnreadWindowRecord>;

const storageKey = "chatmux:unread-windows";

/** Stable per-window identity: the tmux @id survives rename and reorder. */
export function windowStableId(window: Pick<TmuxWindow, "id" | "index">) {
  return window.id || String(window.index);
}

export function unreadWindowKey(hostId: string, sessionName: string, windowId: string) {
  return `${hostId}:${sessionName}:${windowId}`;
}

export function loadUnreadWindows(): UnreadWindows {
  const raw = localStorage.getItem(storageKey);
  if (!raw) {
    return {};
  }
  const parsed = parseUnreadWindows(raw);
  if (!parsed) {
    localStorage.removeItem(storageKey);
    return {};
  }
  return parsed;
}

export function saveUnreadWindows(value: UnreadWindows) {
  if (Object.keys(value).length === 0) {
    localStorage.removeItem(storageKey);
    return;
  }
  localStorage.setItem(storageKey, JSON.stringify(value));
}

/**
 * Drop records of the given host whose window no longer exists or is running
 * again. Other hosts' records are kept untouched so their unread state
 * survives switching away and back.
 */
export function pruneUnreadWindows(current: UnreadWindows, hostId: string, sessions: TmuxSession[]): UnreadWindows {
  const hostPrefix = `${hostId}:`;
  const liveStatuses = new Map<string, SessionStatus>();
  for (const session of sessions) {
    for (const window of session.windowList) {
      liveStatuses.set(unreadWindowKey(hostId, session.name, windowStableId(window)), window.status);
    }
  }
  const next: UnreadWindows = {};
  let changed = false;
  for (const [key, record] of Object.entries(current)) {
    const status = liveStatuses.get(key);
    if (key.startsWith(hostPrefix) && (status === undefined || status === "running")) {
      changed = true;
      continue;
    }
    next[key] = record;
  }
  return changed ? next : current;
}

function parseUnreadWindows(raw: string): UnreadWindows | null {
  let parsed: unknown;
  try {
    parsed = JSON.parse(raw);
  } catch {
    return null;
  }
  if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
    return null;
  }
  const result: UnreadWindows = {};
  for (const [key, value] of Object.entries(parsed)) {
    if (!isUnreadWindowRecord(value)) {
      return null;
    }
    result[key] = value;
  }
  return result;
}

function isUnreadWindowRecord(value: unknown): value is UnreadWindowRecord {
  if (!value || typeof value !== "object") {
    return false;
  }
  const record = value as Partial<UnreadWindowRecord>;
  return typeof record.processName === "string"
    && typeof record.since === "number"
    && typeof record.status === "string";
}
