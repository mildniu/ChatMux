import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { type Terminal } from "@xterm/xterm";
import { terminalScrollbackLines, terminalScrollbackLinesFromText, type ScrollbackLine } from "./terminal-scrollback-lines";
import "./terminal-scrollback.css";

type TerminalScrollbackOverlayProps = {
  loadEarlier?: ((lines: number) => Promise<string>) | null;
  terminal: Terminal;
};

const initialHistoryLines = 5000;
const historyLineStep = 5000;
const topLoadThresholdPx = 48;

export function TerminalScrollbackOverlay({ loadEarlier, terminal }: TerminalScrollbackOverlayProps) {
  const overlayRef = useRef<HTMLDivElement | null>(null);
  const exhaustedRef = useRef(false);
  const historyTextRef = useRef("");
  const loadingRef = useRef(false);
  const loadedHistoryLinesRef = useRef(0);
  const usingHistoryRef = useRef(false);
  // scrollHeight captured *before* a history fetch; after React commits the
  // newly prepended rows we add the height delta back to the current scrollTop.
  const pendingScrollAdjustRef = useRef(false);
  const previousScrollHeightRef = useRef(0);
  const [lines, setLines] = useState(() => terminalScrollbackLines(terminal));
  const [loadError, setLoadError] = useState("");
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    terminal.blur();
    const update = () => {
      if (!usingHistoryRef.current) {
        setLines(terminalScrollbackLines(terminal));
      }
    };
    update();
    const disposable = terminal.onWriteParsed(update);
    return () => disposable.dispose();
  }, [terminal]);

  useEffect(() => {
    const overlay = overlayRef.current;
    if (!overlay) {
      return;
    }
    overlay.scrollTop = overlay.scrollHeight;
    if (overlay.scrollTop <= topLoadThresholdPx) {
      void loadEarlierHistory();
    }
  }, []);

  // Restore the scroll anchor after older history is prepended. Must run *after*
  // React commits the new rows (and before paint) so scrollHeight reflects them;
  // a requestAnimationFrame raced with the commit and left the view pinned at
  // the top. We add the prepended height to the *current* scrollTop (not a value
  // captured when the fetch started): the fetch is async, so the user may have
  // scrolled meanwhile, and anchoring on the stale start position yanks them
  // back to the top ("jump back"). overflow-anchor is disabled on the overlay,
  // so the browser leaves scrollTop untouched across the commit for us to adjust.
  useLayoutEffect(() => {
    if (!pendingScrollAdjustRef.current) {
      return;
    }
    pendingScrollAdjustRef.current = false;
    loadingRef.current = false;
    const overlay = overlayRef.current;
    if (!overlay) {
      return;
    }
    overlay.scrollTop += overlay.scrollHeight - previousScrollHeightRef.current;
    if (overlay.scrollTop <= topLoadThresholdPx && !exhaustedRef.current) {
      void loadEarlierHistory();
    }
  }, [lines]);

  async function loadEarlierHistory() {
    const overlay = overlayRef.current;
    if (!overlay || !loadEarlier || loadingRef.current || exhaustedRef.current) {
      return;
    }
    const nextLineCount = nextHistoryLineCount(loadedHistoryLinesRef.current);
    loadingRef.current = true;
    setLoading(true);
    previousScrollHeightRef.current = overlay.scrollHeight;
    try {
      const historyText = await loadEarlier(nextLineCount);
      exhaustedRef.current = loadedHistoryLinesRef.current > 0 && historyText === historyTextRef.current;
      historyTextRef.current = historyText;
      const parsedLines = await terminalScrollbackLinesFromText(historyText, terminal, nextLineCount);
      loadedHistoryLinesRef.current = nextLineCount;
      usingHistoryRef.current = true;
      setLoadError("");
      // Clear the loading indicator together with the new rows so the scroll
      // measurement below excludes it; restoring the anchor is deferred to the
      // useLayoutEffect above, which runs after these rows reach the DOM.
      setLines(parsedLines);
      setLoading(false);
      pendingScrollAdjustRef.current = true;
    } catch (error) {
      loadingRef.current = false;
      setLoading(false);
      setLoadError(errorMessage(error));
    }
  }

  function handleScroll() {
    const overlay = overlayRef.current;
    if (overlay && overlay.scrollTop <= topLoadThresholdPx) {
      void loadEarlierHistory();
    }
  }

  return (
    <div
      className="terminal-scrollback-overlay"
      ref={overlayRef}
      aria-label="Scrollable terminal output"
      onScroll={handleScroll}
      onTouchEnd={handleScroll}
      onWheel={handleScroll}
    >
      {loading ? <div className="terminal-scrollback-loading">Loading earlier output...</div> : null}
      {loadError ? <div className="terminal-scrollback-error">{loadError}</div> : null}
      {lines.length ? lines.map((line) => <ScrollbackRow key={line.key} line={line} />) : <div>No terminal output yet.</div>}
    </div>
  );
}

function ScrollbackRow({ line }: { line: ScrollbackLine }) {
  return (
    <div className="terminal-scrollback-row">
      {line.segments.map((segment) => (
        <span key={segment.key} style={segment.style}>
          {segment.text}
        </span>
      ))}
    </div>
  );
}

function nextHistoryLineCount(current: number) {
  if (current <= 0) {
    return initialHistoryLines;
  }
  return current + historyLineStep;
}

function errorMessage(error: unknown) {
  if (error instanceof Error) {
    return error.message;
  }
  return String(error);
}
