import type { Metadata } from "next";
import { Suspense } from "react";

import { ProjectDetailRoute } from "@/components/projects/ProjectDetailRoute";

export const metadata: Metadata = { title: "Project — Reharvester" };

/**
 * The project id travels in the query string rather than the path.
 *
 * With `output: "export"` a dynamic `[id]` segment can only serve ids that
 * `generateStaticParams` knew at build time, and a real project id is a task id
 * minted at runtime — so every genuine project 404'd in the exported build.
 */
export default function ProjectDetailPage() {
  return (
    <Suspense fallback={null}>
      <ProjectDetailRoute />
    </Suspense>
  );
}
