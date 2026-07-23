"use client";
import Chip from "@mui/material/Chip";
import Link from "next/link";

import { LlmWarningBadge } from "@/components/system/LlmWarningBadge";
import { useAppStore } from "@/store";

import { StatusChip } from "./StatusChip";


export function AppHeader() {
  const isCrawlActive = useAppStore((s) => s.isCrawlActive);
  const crawlProgress = useAppStore((s) => s.crawlProgress);

  return (
    <header className="flex h-14 shrink-0 items-center justify-between border-b border-line bg-white px-6">
      <div className="flex items-center">
        {isCrawlActive ? (
          <Link href="/" aria-label="Crawl in progress — go to Harvest">
            <Chip
              size="small"
              variant="outlined"
              className="font-mono"
              icon={<span className="h-2 w-2 animate-pulse rounded-full bg-ok" />}
              label={`Crawling · ${crawlProgress != null ? `${crawlProgress}%` : "…"}`}
            />
          </Link>
        ) : null}
      </div>
      <div className="flex items-center gap-3">
        <LlmWarningBadge />
        <StatusChip />
      </div>
    </header>
  );
}
