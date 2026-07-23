"use client";

import Chip from "@mui/material/Chip";
import Tab from "@mui/material/Tab";
import Tabs from "@mui/material/Tabs";
import Typography from "@mui/material/Typography";
import Link from "next/link";
import { useState } from "react";

import { CorpusBrowser } from "@/components/corpus/CorpusSection";
import { CrawlLogTerminal } from "@/components/ingestion/CrawlLogTerminal";
import { GraphExplorer } from "@/components/graph/GraphSection";
import { SectionShell } from "@/components/layout/SectionShell";
import { formatRelative } from "@/lib/relative-time";
import { useProjectsStore } from "@/store/projects";

type TabKey = "graph" | "corpus" | "log";


export function ProjectDetail({ id }: { id: string }) {
  const project = useProjectsStore((s) => s.projects.find((p) => p.id === id));
  const [tab, setTab] = useState<TabKey>("graph");

  if (!project) {
    return (
      <SectionShell id="project" eyebrow="Project" title="Not found">
        <Typography variant="body2" className="text-ink-2">
          No project with id <span className="font-mono">{id}</span>.{" "}
          <Link href="/" className="text-accent underline">
            Back to Harvest
          </Link>
        </Typography>
      </SectionShell>
    );
  }

  return (
    <SectionShell
      id="project"
      eyebrow="Project"
      title={project.name}
      description={project.query}
      headerAside={
        <div className="flex items-center gap-3">
          <Chip
            size="small"
            variant="outlined"
            label={project.status === "crawling" ? "Crawling" : project.status === "failed" ? "Failed" : "Complete"}
          />
          <span className="font-mono text-[13px] text-ink-2">{project.docsIngested} docs</span>
          <span className="text-[13px] text-ink-3">{formatRelative(project.createdAt)}</span>
        </div>
      }
    >
      {}
      <div className="border-b border-line">
        <Tabs value={tab} onChange={(_, v: TabKey) => setTab(v)} aria-label="Project views">
          <Tab value="graph" label="Graph" />
          <Tab value="corpus" label="Corpus" />
          <Tab value="log" label="Crawl log" />
        </Tabs>
      </div>

      {tab === "graph" ? <GraphExplorer /> : null}
      {tab === "corpus" ? <CorpusBrowser /> : null}
      {tab === "log" ? (
        project.logSnapshot.length > 0 ? (
          <CrawlLogTerminal entries={project.logSnapshot} />
        ) : (
          <Typography variant="caption" className="text-ink-2">
            No log captured for this project yet
            {project.status === "crawling" ? " — the crawl is still running." : "."}
          </Typography>
        )
      ) : null}
    </SectionShell>
  );
}
