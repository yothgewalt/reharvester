"use client";
import { useSearchParams } from "next/navigation";

import { ProjectDetail } from "./ProjectDetail";

export function ProjectDetailRoute() {
  const id = useSearchParams().get("id") ?? "";
  return <ProjectDetail id={id} />;
}
