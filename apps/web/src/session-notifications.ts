import { Capacitor } from "@capacitor/core";
import { LocalNotifications } from "@capacitor/local-notifications";
import { isBrowserShell } from "./runtime-platform";
import { type WindowFinishedEvent } from "./window-transitions";

export const notificationClickMessageType = "chatmux:notification-click";

export type WindowFinishedNotificationArgs = {
  event: WindowFinishedEvent;
  hostName: string;
  onOpenWindow?: (sessionName: string, windowIndex: number) => void;
};

const notificationChannelId = "chatmux-session-status";
const notificationGroupId = "chatmux-session-updates";
const notificationHashMultiplier = 31;
const notificationIdBase = 1000;
const notificationIdModulo = 1_000_000_000;
const notificationImportanceDefault = 3;
const notificationVisibilityPrivate = 0;

export async function ensureSessionNotificationPermission() {
  if (Capacitor.isNativePlatform()) {
    await ensureNativeNotificationPermission();
    return;
  }
  await ensureWebNotificationPermission();
}

/** Whether notification permission is already granted (never prompts). */
export async function sessionNotificationPermissionGranted() {
  if (Capacitor.isNativePlatform()) {
    const current = await LocalNotifications.checkPermissions();
    return current.display === "granted";
  }
  return "Notification" in window && Notification.permission === "granted";
}

export async function sendWindowFinishedNotification(args: WindowFinishedNotificationArgs) {
  const payload = windowFinishedPayload(args.hostName, args.event);
  if (Capacitor.isNativePlatform()) {
    await sendNativeNotification(payload);
    return;
  }
  await sendWebNotification(payload, args);
}

type WindowNotificationPayload = ReturnType<typeof windowFinishedPayload>;

function windowFinishedPayload(hostName: string, event: WindowFinishedEvent) {
  const scope = `${event.hostId}:${event.sessionName}:${event.windowId}`;
  const verb = event.nextStatus === "failed" ? "failed" : "finished";
  return {
    body: `${event.sessionTitle || event.sessionName} · ${event.windowLabel} — ${hostName}`,
    data: {
      hostId: event.hostId,
      sessionName: event.sessionName,
      type: notificationClickMessageType,
      windowIndex: event.windowIndex,
    },
    id: notificationId(scope),
    // One notification per window: a newer finish replaces the older one
    // instead of stacking up.
    tag: `chatmux:${scope}`,
    title: `${event.processName} ${verb}`,
  };
}

async function ensureNativeNotificationPermission() {
  const current = await LocalNotifications.checkPermissions();
  const next = current.display === "granted" ? current : await LocalNotifications.requestPermissions();
  if (next.display !== "granted") {
    throw new Error("Notification permission was denied");
  }
  await createAndroidNotificationChannel();
}

async function createAndroidNotificationChannel() {
  if (Capacitor.getPlatform() !== "android") {
    return;
  }
  await LocalNotifications.createChannel({
    description: "ChatMux tmux session state changes",
    id: notificationChannelId,
    importance: notificationImportanceDefault,
    name: "Session updates",
    visibility: notificationVisibilityPrivate,
  });
}

async function ensureWebNotificationPermission() {
  if (!("Notification" in window)) {
    throw new Error("Notifications are not available in this browser");
  }
  const permission = Notification.permission === "granted" ? "granted" : await Notification.requestPermission();
  if (permission !== "granted") {
    throw new Error("Notification permission was denied");
  }
}

async function sendNativeNotification(payload: WindowNotificationPayload) {
  await LocalNotifications.schedule({
    notifications: [{
      autoCancel: true,
      body: payload.body,
      channelId: notificationChannelId,
      extra: payload.data,
      group: notificationGroupId,
      id: payload.id,
      title: payload.title,
    }],
  });
}

async function sendWebNotification(payload: WindowNotificationPayload, args: WindowFinishedNotificationArgs) {
  if (Notification.permission !== "granted") {
    throw new Error("Notification permission was denied");
  }
  const options: NotificationOptions = {
    body: payload.body,
    data: payload.data,
    tag: payload.tag,
  };
  const registration = await webNotificationServiceWorkerRegistration();
  if (registration) {
    // Click handling lives in the service worker's notificationclick handler,
    // which posts the payload data back to the page.
    await registration.showNotification(payload.title, options);
    return;
  }
  const notification = new Notification(payload.title, options);
  notification.onclick = () => {
    window.focus();
    args.onOpenWindow?.(args.event.sessionName, args.event.windowIndex);
  };
}

async function webNotificationServiceWorkerRegistration() {
  if (!("serviceWorker" in navigator)) {
    return null;
  }
  const registration = await navigator.serviceWorker.getRegistration();
  if (isNotificationRegistration(registration)) {
    return registration;
  }
  if (import.meta.env.DEV || !isBrowserShell()) {
    return null;
  }
  return notificationRegistration(await navigator.serviceWorker.register("/service-worker.js"));
}

function notificationRegistration(registration: ServiceWorkerRegistration) {
  if (!isNotificationRegistration(registration)) {
    throw new Error("Service worker notifications are not available in this browser");
  }
  return registration;
}

function isNotificationRegistration(registration: ServiceWorkerRegistration | undefined) {
  return Boolean(registration && typeof registration.showNotification === "function");
}

function notificationId(value: string) {
  let hash = 0;
  for (const char of value) {
    hash = (hash * notificationHashMultiplier + char.charCodeAt(0)) | 0;
  }
  return Math.abs(hash % notificationIdModulo) + notificationIdBase;
}
