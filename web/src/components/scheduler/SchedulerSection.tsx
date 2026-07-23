"use client";

import { SectionShell } from "@/components/layout/SectionShell";

import { ActiveCrawlers } from "./ActiveCrawlers";
import { SchedulerForm } from "./SchedulerForm";

export function SchedulerSection() {
  return (
    <SectionShell
      id="scheduler"
      eyebrow="Automation"
      title="Scheduled crawlers"
      tone="white"
      description="Register recurring harvest profiles — each cadence re-runs the crawler and folds new results into the graph."
    >
      <div className="grid grid-cols-1 gap-8 lg:grid-cols-2">
        <SchedulerForm />
        <ActiveCrawlers />
      </div>
    </SectionShell>
  );
}
