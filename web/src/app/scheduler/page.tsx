import type { Metadata } from "next";

import { SchedulerSection } from "@/components/scheduler/SchedulerSection";

export const metadata: Metadata = { title: "Scheduler — Reharvester" };

export default function SchedulerPage() {
  return <SchedulerSection />;
}
