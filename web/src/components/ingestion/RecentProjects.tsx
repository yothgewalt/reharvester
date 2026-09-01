"use client";

import Chip from "@mui/material/Chip";
import Typography from "@mui/material/Typography";
import Link from "next/link";

import { formatRelative } from "@/lib/relative-time";
import { useProjectsStore } from "@/store/projects";
import type { Project } from "@/types/domain";

const VISIBLE = 5;

export function ProjectStatusChip({ status }: { status: Project["status"] }) {
  if (status === "crawling") {
    return (
      <Chip
        size="small"
        variant="outlined"
        icon={<span className="h-2 w-2 animate-pulse rounded-full bg-ok" />}
        label="Crawling"
      />
    );
  }
  if (status === "failed") {
    return (
      <Chip
        size="small"
        variant="outlined"
        icon={<span className="h-2 w-2 rounded-full bg-error" />}
        label="Failed"
      />
    );
  }
  return <Chip size="small" variant="outlined" label="Complete" />;
}


export function ProjectRow({ project }: { project: Project }) {
  return (
    <Link
      href={`/projects/detail?id=${encodeURIComponent(project.id)}`}
      className="flex items-center justify-between gap-4 border-t border-line py-3 px-3 transition-colors duration-150 first:border-t-0 hover:bg-bg-faint"
    >
      <div className="flex min-w-0 flex-col">
        <span className="truncate text-sm font-medium text-ink">{project.name}</span>
        <span className="truncate text-[13px] text-ink-2">{project.query}</span>
      </div>
      <div className="flex shrink-0 items-center gap-3">
        <ProjectStatusChip status={project.status} />
        <span className="font-mono text-[13px] text-ink-2">{project.docsIngested} docs</span>
        <span className="w-24 text-right text-[13px] text-ink-3">
          {formatRelative(project.createdAt)}
        </span>
      </div>
    </Link>
  );
}


export function RecentProjects() {
  const projects = useProjectsStore((s) => s.projects);

  return (
    <div className="flex w-full flex-col gap-1">
      <Typography variant="overline" className="text-ink-3">
        Recent projects
      </Typography>
      {projects.length === 0 ? (
        <Typography variant="body2" className="text-ink-2">
          No projects have been created yet.
        </Typography>
      ) : (
        <div className="flex flex-col">
          {projects.slice(0, VISIBLE).map((p) => (
            <ProjectRow key={p.id} project={p} />
          ))}
        </div>
      )}
    </div>
  );
}
