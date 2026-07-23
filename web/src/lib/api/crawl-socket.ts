import { WS_BASE } from "@/lib/config";
import type { CrawlStreamEvent, WsStatus } from "@/types/domain";

import { reportBackendFailure } from "./failure-bus";
import type { CrawlSocketHandle } from "./transport";

const BASE_DELAY_MS = 1_000;
const MAX_DELAY_MS = 30_000;
const MAX_ATTEMPTS = 6;


export function openReconnectingCrawlSocket(
  taskId: string,
  onEvent: (e: CrawlStreamEvent) => void,
  onStatus: (s: WsStatus) => void,
): CrawlSocketHandle {
  let ws: WebSocket | null = null;
  let closedByUser = false;
  let doneSeen = false;
  let attempt = 0;
  let retryTimer: ReturnType<typeof setTimeout> | null = null;

  const connect = () => {
    onStatus(attempt === 0 ? "connecting" : "reconnecting");
    ws = new WebSocket(`${WS_BASE}/api/v1/harvest/stream/${encodeURIComponent(taskId)}`);

    ws.onopen = () => {


      onStatus("open");
    };

    ws.onmessage = (msg) => {
      attempt = 0;
      let event: CrawlStreamEvent;
      try {
        event = JSON.parse(String(msg.data)) as CrawlStreamEvent;
      } catch {
        return;
      }
      if (event.type === "done") doneSeen = true;
      onEvent(event);
    };

    ws.onclose = () => {
      if (closedByUser || doneSeen) {
        onStatus("closed");
        return;
      }
      attempt += 1;
      if (attempt >= 3) reportBackendFailure();
      if (attempt >= MAX_ATTEMPTS) {
        onEvent({
          type: "log",
          entry: {
            id: `socket-terminal-${Date.now()}`,
            timestamp: new Date().toISOString(),
            level: "error",
            message: `Crawl stream lost after ${MAX_ATTEMPTS} reconnect attempts — giving up.`,
          },
        });
        onStatus("closed");
        return;
      }
      const delay = Math.min(BASE_DELAY_MS * 2 ** (attempt - 1), MAX_DELAY_MS);
      const jitter = delay * 0.2 * (Math.random() * 2 - 1);
      retryTimer = setTimeout(connect, delay + jitter);
      onStatus("reconnecting");
    };

    ws.onerror = () => {
      ws?.close();
    };
  };

  connect();

  return {
    close() {
      closedByUser = true;
      if (retryTimer) clearTimeout(retryTimer);
      ws?.close();
      onStatus("closed");
    },
  };
}
