"use client";

import { SectionShell } from "@/components/layout/SectionShell";
import { useAppStore } from "@/store";

import { CrawlLogTerminal } from "./CrawlLogTerminal";
import { CrawlProgress } from "./CrawlProgress";
import { IngestionForm } from "./IngestionForm";
import { RecentProjects } from "./RecentProjects";

export function IngestionSection() {
  const hasLogs = useAppStore((s) => s.consoleLogHistory.length > 0);

  return (
    <SectionShell
      id="ingest"
      eyebrow="Local harvester"
      title="Ingestion"
      tone="white"
      description="Seed the graph with comma-separated keywords or a full abstract — the local harvester crawls and links the rest."
    >
      {}
      <div className="flex w-full flex-col gap-4">
        <IngestionForm />
        <CrawlProgress />
        {hasLogs ? <CrawlLogTerminal /> : null}
        <RecentProjects />
      </div>
    </SectionShell>
  );
}
