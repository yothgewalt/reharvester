import type { CrawlSocketHandle } from "@/lib/api/transport";
import type { CrawlStreamEvent, ProjectAction, WsStatus } from "@/types/domain";

import { buildActionScript, buildCrawlScript } from "./fixtures/crawl-script";


export class MockCrawlSocket implements CrawlSocketHandle {
  private timers: Array<ReturnType<typeof setTimeout>> = [];
  private closed = false;

  constructor(
    query: string,
    onEvent: (e: CrawlStreamEvent) => void,
    private onStatus: (s: WsStatus) => void,
    action?: ProjectAction,
  ) {
    onStatus("connecting");
    this.timers.push(
      setTimeout(() => {
        if (this.closed) return;
        onStatus("open");
        const steps = action ? buildActionScript(action, query) : buildCrawlScript(query);
        let acc = 0;
        for (const step of steps) {
          acc += step.delayMs;
          this.timers.push(
            setTimeout(() => {
              if (!this.closed) onEvent(step.event);
            }, acc),
          );
        }
      }, 300),
    );
  }

  close(): void {
    this.closed = true;
    this.timers.forEach(clearTimeout);
    this.timers = [];
    this.onStatus("closed");
  }
}
