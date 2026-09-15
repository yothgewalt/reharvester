"use client";
import { useSearchParams } from "next/navigation";

import { isProjectTab, ProjectDetail } from "./ProjectDetail";

export function ProjectDetailRoute() {
  const params = useSearchParams();
  const tab = params.get("tab");
  return (
    <ProjectDetail
      id={params.get("id") ?? ""}
      initialTab={tab && isProjectTab(tab) ? tab : undefined}
      pinDocId={params.get("pin") ?? undefined}
    />
  );
}
