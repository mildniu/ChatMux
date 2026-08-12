import { invoke } from "@tauri-apps/api/core";
import { listen, type UnlistenFn } from "@tauri-apps/api/event";
import { getCurrentWindow } from "@tauri-apps/api/window";
import { useCallback, useEffect, useRef, useState } from "react";
import { isDesktopShell } from "./runtime-platform";

export type CloseBehaviorState = {
  dialogVisible: boolean;
  handleExit: (remember: boolean) => void;
  handleMinimize: (remember: boolean) => void;
  handleCancelDialog: () => void;
};

export function useCloseBehavior(): CloseBehaviorState {
  const [dialogVisible, setDialogVisible] = useState(false);
  const unlistenRef = useRef<UnlistenFn | null>(null);

  // Listen for close-requested event from Rust
  useEffect(() => {
    if (!isDesktopShell()) {
      return;
    }

    let cancelled = false;

    (async () => {
      const un = await listen("close-requested", async () => {
        // Check if user has a saved preference
        try {
          const saved = await invoke<string>("get_close_behavior");
          if (saved === "exit") {
            await getCurrentWindow().destroy();
            return;
          }
          if (saved === "minimize") {
            await getCurrentWindow().hide();
            return;
          }
        } catch {
          // Ignore errors, fall through to dialog
        }
        // No saved preference, show dialog
        setDialogVisible(true);
      });
      if (!cancelled) {
        unlistenRef.current = un;
      } else {
        un();
      }
    })();

    return () => {
      cancelled = true;
      unlistenRef.current?.();
      unlistenRef.current = null;
    };
  }, []);

  const handleExit = useCallback(async (remember: boolean) => {
    if (remember && isDesktopShell()) {
      try {
        await invoke("set_close_behavior", { behavior: "exit" });
      } catch {
        // Ignore write errors
      }
    }
    setDialogVisible(false);
    if (isDesktopShell()) {
      await getCurrentWindow().destroy();
    }
  }, []);

  const handleMinimize = useCallback(async (remember: boolean) => {
    if (remember && isDesktopShell()) {
      try {
        await invoke("set_close_behavior", { behavior: "minimize" });
      } catch {
        // Ignore write errors
      }
    }
    setDialogVisible(false);
    if (isDesktopShell()) {
      await getCurrentWindow().hide();
    }
  }, []);

  const handleCancelDialog = useCallback(() => {
    setDialogVisible(false);
  }, []);

  return {
    dialogVisible,
    handleExit,
    handleMinimize,
    handleCancelDialog,
  };
}
