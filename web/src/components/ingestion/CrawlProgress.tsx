"use client";

import LinearProgress from "@mui/material/LinearProgress";

import { useAppStore } from "@/store";

export function CrawlProgress() {
  const isCrawlActive = useAppStore((s) => s.isCrawlActive);
  const crawlProgress = useAppStore((s) => s.crawlProgress);
  const crawlStage = useAppStore((s) => s.crawlStage);
  const activeTask = useAppStore((s) => s.activeTask);

  if (!isCrawlActive && crawlProgress === null) return null;

  return (
    <div className="flex flex-col gap-2">
      <LinearProgress
        variant={crawlProgress != null ? "determinate" : "indeterminate"}
        value={crawlProgress ?? 0}
      />
      <div className="flex items-baseline justify-between gap-4">
        <span className="shrink-0 font-mono text-[13px] text-ink-2">
          {crawlStage ?? "starting …"}
        </span>
        {activeTask ? (
          <span className="min-w-0 truncate text-[13px] text-ink-2">{activeTask}</span>
        ) : null}
      </div>
    </div>
  );
}
