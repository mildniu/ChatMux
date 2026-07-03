import { useCallback, useEffect, useRef, useState } from "react";
import {
  ensureSessionNotificationPermission,
  notificationClickMessageType,
  sendWindowFinishedNotification,
  sessionNotificationPermissionGranted,
} from "./session-notifications";
import { type WindowFinishedEvent } from "./window-transitions";
import { errorMessage } from "./view-utils";

export type SessionNotificationStatus = "credential-error" | "credential-needed" | "denied" | "enabling" | "off" | "watching";

type SessionNotificationsOptions = {
  hostId: string;
  hostName: string;
  sshReady: boolean;
  onError: (message: string) => void;
  onOpenWindow: (sessionName: string, windowIndex: number) => void;
};

const alertsStorageKey = "chatmux:session-alerts";

function readStoredAlertsEnabled() {
  return localStorage.getItem(alertsStorageKey) === "on";
}

function persistAlertsEnabled(enabled: boolean) {
  localStorage.setItem(alertsStorageKey, enabled ? "on" : "off");
}

export function useSessionNotifications(options: SessionNotificationsOptions) {
  const [enabled, setEnabledState] = useState(readStoredAlertsEnabled);
  const [enabling, setEnabling] = useState(false);
  const [permissionDenied, setPermissionDenied] = useState(false);
  const [credentialError, setCredentialError] = useState(false);
  const status = notificationStatus({
    credentialError,
    enabled,
    enabling,
    permissionDenied,
    ready: Boolean(options.hostId) && options.sshReady,
  });
  const statusRef = useRef(status);
  statusRef.current = status;

  // On load with alerts stored on, verify the permission is still granted
  // without prompting; if the user revoked it, surface "denied" in the UI.
  useEffect(() => {
    if (!enabled) {
      return;
    }
    let active = true;
    void sessionNotificationPermissionGranted().then((granted) => {
      if (active && !granted) {
        setPermissionDenied(true);
      }
    });
    return () => {
      active = false;
    };
  }, [enabled]);

  useNotificationClickMessages(options.hostId, options.onOpenWindow);

  const setEnabled = useCallback(async (nextEnabled: boolean) => {
    await updateNotificationEnabled(nextEnabled, {
      setEnabledState,
      setEnabling,
      setPermissionDenied,
      onError: options.onError,
    });
  }, [options.onError]);

  const notifyFinished = useCallback((events: WindowFinishedEvent[]) => {
    if (statusRef.current !== "watching") {
      return;
    }
    for (const event of events) {
      void sendWindowFinishedNotification({
        event,
        hostName: options.hostName,
        onOpenWindow: options.onOpenWindow,
      }).catch((error) => options.onError(errorMessage(error)));
    }
  }, [options.hostName, options.onError, options.onOpenWindow]);

  const markRefreshError = useCallback(() => setCredentialError(true), []);
  const markRefreshSuccess = useCallback(() => setCredentialError(false), []);

  return { enabled, markRefreshError, markRefreshSuccess, notifyFinished, setEnabled, status };
}

function notificationStatus(state: {
  credentialError: boolean;
  enabled: boolean;
  enabling: boolean;
  permissionDenied: boolean;
  ready: boolean;
}): SessionNotificationStatus {
  if (state.enabling) {
    return "enabling";
  }
  if (state.permissionDenied) {
    return "denied";
  }
  if (!state.enabled) {
    return "off";
  }
  if (!state.ready) {
    return "credential-needed";
  }
  if (state.credentialError) {
    return "credential-error";
  }
  return "watching";
}

async function updateNotificationEnabled(enabled: boolean, actions: {
  setEnabledState: (enabled: boolean) => void;
  setEnabling: (enabling: boolean) => void;
  setPermissionDenied: (denied: boolean) => void;
  onError: (message: string) => void;
}) {
  if (!enabled) {
    persistAlertsEnabled(false);
    actions.setEnabledState(false);
    actions.setPermissionDenied(false);
    return;
  }
  actions.setEnabling(true);
  try {
    await ensureSessionNotificationPermission();
    persistAlertsEnabled(true);
    actions.setEnabledState(true);
    actions.setPermissionDenied(false);
    actions.onError("");
  } catch (error) {
    persistAlertsEnabled(false);
    actions.setEnabledState(false);
    actions.setPermissionDenied(true);
    actions.onError(errorMessage(error));
  } finally {
    actions.setEnabling(false);
  }
}

/**
 * The service worker posts the notification's data back to the page when a
 * notification is clicked; open the matching window.
 */
function useNotificationClickMessages(hostId: string, onOpenWindow: (sessionName: string, windowIndex: number) => void) {
  useEffect(() => {
    if (!("serviceWorker" in navigator)) {
      return;
    }
    const handleMessage = (event: MessageEvent) => {
      const data: unknown = event.data;
      if (!isNotificationClickMessage(data) || data.hostId !== hostId) {
        return;
      }
      onOpenWindow(data.sessionName, data.windowIndex);
    };
    navigator.serviceWorker.addEventListener("message", handleMessage);
    return () => navigator.serviceWorker.removeEventListener("message", handleMessage);
  }, [hostId, onOpenWindow]);
}

type NotificationClickMessage = {
  hostId: string;
  sessionName: string;
  type: string;
  windowIndex: number;
};

function isNotificationClickMessage(value: unknown): value is NotificationClickMessage {
  if (!value || typeof value !== "object") {
    return false;
  }
  const message = value as Partial<NotificationClickMessage>;
  return message.type === notificationClickMessageType
    && typeof message.hostId === "string"
    && typeof message.sessionName === "string"
    && typeof message.windowIndex === "number";
}
