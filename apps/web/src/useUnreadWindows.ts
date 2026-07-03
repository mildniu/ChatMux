import { useCallback, useEffect, useMemo, useState } from "react";
import { type TmuxSession } from "./api";
import { findSessionWindow } from "./session-window-utils";
import {
  loadUnreadWindows,
  pruneUnreadWindows,
  saveUnreadWindows,
  unreadWindowKey,
  windowStableId,
  type UnreadWindows,
} from "./unread-windows";
import { type WindowFinishedEvent } from "./window-transitions";

export type ViewedWindow = {
  hostId: string;
  sessionName: string;
  terminalVisible: boolean;
  windowIndex: number | null;
};

/**
 * A window only counts as "being viewed" when its terminal is on screen in a
 * visible, focused document — the same gate that suppresses notifications and
 * unread marks, and that clears an unread mark on sight.
 */
export function windowIsViewed(viewed: ViewedWindow, sessionName: string, windowIndex: number) {
  return viewed.terminalVisible
    && viewed.sessionName === sessionName
    && viewed.windowIndex === windowIndex
    && document.visibilityState === "visible"
    && document.hasFocus();
}

export function useUnreadWindows(options: { hostId: string; sessions: TmuxSession[]; viewed: ViewedWindow }) {
  const [unread, setUnread] = useState<UnreadWindows>(loadUnreadWindows);

  useEffect(() => {
    saveUnreadWindows(unread);
  }, [unread]);

  useEffect(() => {
    if (!options.hostId) {
      return;
    }
    setUnread((current) => pruneUnreadWindows(current, options.hostId, options.sessions));
  }, [options.hostId, options.sessions]);

  const clearViewedWindow = useCallback(() => {
    const { hostId, sessionName, windowIndex } = options.viewed;
    if (!hostId || !sessionName || windowIndex === null || !windowIsViewed(options.viewed, sessionName, windowIndex)) {
      return;
    }
    const session = options.sessions.find((candidate) => candidate.name === sessionName);
    const selectedWindow = findSessionWindow(session, windowIndex);
    if (!selectedWindow) {
      return;
    }
    const key = unreadWindowKey(hostId, sessionName, windowStableId(selectedWindow));
    setUnread((current) => (key in current ? removeUnreadKey(current, key) : current));
  }, [
    options.sessions,
    options.viewed.hostId,
    options.viewed.sessionName,
    options.viewed.terminalVisible,
    options.viewed.windowIndex,
  ]);

  useEffect(() => {
    clearViewedWindow();
    window.addEventListener("focus", clearViewedWindow);
    document.addEventListener("visibilitychange", clearViewedWindow);
    return () => {
      window.removeEventListener("focus", clearViewedWindow);
      document.removeEventListener("visibilitychange", clearViewedWindow);
    };
  }, [clearViewedWindow]);

  const markUnread = useCallback((events: WindowFinishedEvent[]) => {
    if (events.length === 0) {
      return;
    }
    const now = Date.now();
    setUnread((current) => {
      const next = { ...current };
      for (const event of events) {
        next[unreadWindowKey(event.hostId, event.sessionName, event.windowId)] = {
          processName: event.processName,
          since: now,
          status: event.nextStatus,
        };
      }
      return next;
    });
  }, []);

  const unreadKeys = useMemo(() => new Set(Object.keys(unread)), [unread]);
  const hostPrefix = `${options.hostId}:`;
  const unreadCount = useMemo(
    () => Object.keys(unread).filter((key) => key.startsWith(hostPrefix)).length,
    [hostPrefix, unread],
  );

  return { markUnread, unreadCount, unreadKeys };
}

const baseDocumentTitle = "ChatMux";

/** Surface the unread count in the tab title, e.g. "(2) ChatMux". */
export function useUnreadDocumentTitle(unreadCount: number) {
  useEffect(() => {
    document.title = unreadCount > 0 ? `(${unreadCount}) ${baseDocumentTitle}` : baseDocumentTitle;
  }, [unreadCount]);
}

function removeUnreadKey(current: UnreadWindows, key: string): UnreadWindows {
  const { [key]: _removed, ...rest } = current;
  return rest;
}
