import type { Metadata } from "next";

import { ProjectsSection } from "@/components/projects/ProjectsSection";

export const metadata: Metadata = { title: "Projects — Reharvester" };

export default function ProjectsPage() {
  return <ProjectsSection />;
}
