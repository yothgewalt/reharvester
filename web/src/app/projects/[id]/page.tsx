import type { Metadata } from "next";

import { ProjectDetail } from "@/components/projects/ProjectDetail";
import { MOCK_PROJECTS } from "@/mocks/fixtures/projects";

export const metadata: Metadata = { title: "Project — Reharvester" };

export function generateStaticParams() {
  return MOCK_PROJECTS.map((project) => ({ id: project.id }));
}

export default async function ProjectPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return <ProjectDetail id={id} />;
}
