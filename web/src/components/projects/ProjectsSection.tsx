"use client";

import Button from "@mui/material/Button";
import Typography from "@mui/material/Typography";
import Link from "next/link";

import { ProjectRow } from "@/components/ingestion/RecentProjects";
import { SectionShell } from "@/components/layout/SectionShell";
import { useProjectsStore } from "@/store/projects";


export function ProjectsSection() {
  const projects = useProjectsStore((s) => s.projects);

  return (
    <SectionShell
      id="projects"
      eyebrow="Harvest runs"
      title="Projects"
      description="Each harvester run becomes a project — graph, corpus, and crawl log scoped in one place."
    >
      {projects.length === 0 ? (
        <div className="flex flex-col items-start gap-3">
          <Typography variant="body2" className="text-ink-2">
            No projects yet — run the harvester to create your first one.
          </Typography>
          <Link href="/">
            <Button variant="contained">Go to Harvest</Button>
          </Link>
        </div>
      ) : (
        <div className="flex flex-col">
          {projects.map((p) => (
            <ProjectRow key={p.id} project={p} />
          ))}
        </div>
      )}
    </SectionShell>
  );
}
