"use client";

import { useEffect, useRef } from "react";

import { useAppStore } from "@/store";
import type { CrawlLogEntry } from "@/types/domain";

const VISIBLE_CAP = 500;
const BOTTOM_THRESHOLD_PX = 24;

const pad = (n: number) => String(n).padStart(2, "0");

function formatTime(iso: string): string {
  const d = new Date(iso);
  return `${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`;
}

function LevelPrefix({ level }: { level: CrawlLogEntry["level"] }) {

  if (level === "warn") return <span className="mr-2 text-ink-4">WARN</span>;
  if (level === "error") return <span className="mr-2 text-ink-4">ERR</span>;
  if (level === "success") return <span className="mr-2 text-ink-inverse">OK</span>;
  return null;
}

export function CrawlLogTerminal({ entries: entriesProp }: { entries?: CrawlLogEntry[] }) {
  const liveLog = useAppStore((s) => s.consoleLogHistory);
  const consoleLogHistory = entriesProp ?? liveLog;
  const containerRef = useRef<HTMLDivElement>(null);
  const atBottomRef = useRef(true);

  const count = consoleLogHistory.length;


  useEffect(() => {
    const el = containerRef.current;
    if (el && atBottomRef.current) el.scrollTop = el.scrollHeight;
  }, [count]);

  if (count === 0) return null;

  const entries =
    count > VISIBLE_CAP ? consoleLogHistory.slice(-VISIBLE_CAP) : consoleLogHistory;

  const handleScroll = () => {
    const el = containerRef.current;
    if (!el) return;
    atBottomRef.current =
      el.scrollHeight - el.scrollTop - el.clientHeight <= BOTTOM_THRESHOLD_PX;
  };

  return (
    <div
      ref={containerRef}
      onScroll={handleScroll}
      role="log"
      aria-live="polite"
      aria-label="Crawl log"
      className="h-72 overflow-y-auto rounded-lg bg-slab p-4 font-mono text-[13px] leading-relaxed text-ink-inverse ring-line-dark"
    >
      {entries.map((entry) => (
        <div key={entry.id} className="break-words">
          <span className="mr-3 text-ink-4">{formatTime(entry.timestamp)}</span>
          <LevelPrefix level={entry.level} />
          {entry.message}
        </div>
      ))}
    </div>
  );
}
