import { LogOut, Minimize2, X } from "lucide-react";
import { useState } from "react";
import "./close-behavior-dialog.css";

type CloseBehaviorDialogProps = {
  visible: boolean;
  onExit: (remember: boolean) => void;
  onMinimize: (remember: boolean) => void;
  onCancel: () => void;
};

export function CloseBehaviorDialog({ visible, onExit, onMinimize, onCancel }: CloseBehaviorDialogProps) {
  const [remember, setRemember] = useState(false);

  if (!visible) {
    return null;
  }

  return (
    <div className="close-behavior-backdrop" onMouseDown={onCancel}>
      <section
        aria-labelledby="close-behavior-title"
        aria-modal="true"
        className="close-behavior-dialog"
        role="dialog"
        onMouseDown={(event) => event.stopPropagation()}
      >
        <header>
          <div className="close-behavior-title">
            <h2 id="close-behavior-title">Close ChatMux</h2>
          </div>
          <button aria-label="Close dialog" type="button" onClick={onCancel}>
            <X size={18} aria-hidden="true" />
          </button>
        </header>
        <div className="close-behavior-body">
          <p>What would you like to do?</p>
          <label className="close-behavior-remember">
            <input
              type="checkbox"
              checked={remember}
              onChange={(event) => setRemember(event.target.checked)}
            />
            Remember my choice
          </label>
        </div>
        <footer>
          <button
            className="close-behavior-secondary"
            type="button"
            onClick={() => onMinimize(remember)}
          >
            <Minimize2 size={17} aria-hidden="true" />
            Minimize to Tray
          </button>
          <button
            className="close-behavior-danger"
            type="button"
            onClick={() => onExit(remember)}
          >
            <LogOut size={17} aria-hidden="true" />
            Exit
          </button>
        </footer>
      </section>
    </div>
  );
}
