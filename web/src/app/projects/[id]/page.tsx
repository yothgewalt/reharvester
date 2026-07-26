import type { Metadata } from "next";

import { ProjectDetail } from "@/components/projects/ProjectDetail";
import { MOCK_PROJECTS } from "@/mocks/fixtures/projects";

export const metadata: Metadata = { title: "Project — Reharvester" };

// ponytail: static export needs the route's full ID set enumerated; mock data is fixed.
export function generateStaticParams() {
  return MOCK_PROJECTS.map((p) => ({ id: p.id }));
}

export default async function ProjectPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return <ProjectDetail id={id} />;
}
