"use client";

import ArrowDropDown from "@mui/icons-material/ArrowDropDown";
import ScheduleOutlined from "@mui/icons-material/ScheduleOutlined";
import Button from "@mui/material/Button";
import ButtonGroup from "@mui/material/ButtonGroup";
import Chip from "@mui/material/Chip";
import ClickAwayListener from "@mui/material/ClickAwayListener";
import Grow from "@mui/material/Grow";
import MenuItem from "@mui/material/MenuItem";
import MenuList from "@mui/material/MenuList";
import Paper from "@mui/material/Paper";
import Popper from "@mui/material/Popper";
import Tab from "@mui/material/Tab";
import Tabs from "@mui/material/Tabs";
import Typography from "@mui/material/Typography";
import Link from "next/link";
import { useState } from "react";

import { CorpusBrowser } from "@/components/corpus/CorpusSection";
import { CrawlLogTerminal } from "@/components/ingestion/CrawlLogTerminal";
import { GraphExplorer } from "@/components/graph/GraphSection";
import { SectionShell } from "@/components/layout/SectionShell";
import { ActiveCrawlers } from "@/components/scheduler/ActiveCrawlers";
import { SchedulerForm } from "@/components/scheduler/SchedulerForm";
import { formatRelative } from "@/lib/relative-time";
import { useAppStore } from "@/store";
import { useProjectsStore } from "@/store/projects";
import type { ProjectAction } from "@/types/domain";

type TabKey = "graph" | "corpus" | "log" | "scheduler";

const ACTIONS: { value: ProjectAction; label: string }[] = [
  { value: "add", label: "Add" },
  { value: "subtract", label: "Subtract" },
  { value: "extract", label: "Extract" },
  { value: "summarize", label: "Summarize" },
];


export function ProjectDetail({ id }: { id: string }) {
  const project = useProjectsStore((s) => s.projects.find((p) => p.id === id));
  const isCrawlActive = useAppStore((s) => s.isCrawlActive);
  const startProjectAction = useAppStore((s) => s.startProjectAction);
  const [tab, setTab] = useState<TabKey>("graph");
  const [menuOpen, setMenuOpen] = useState(false);
  const [actionButton, setActionButton] = useState<HTMLButtonElement | null>(null);

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
      <div className="flex items-center justify-between gap-4 border-b border-line">
        <Tabs
          value={tab}
          onChange={(_, v: TabKey) => setTab(v)}
          aria-label="Project views"
          sx={{ "& .MuiTab-root": { p: 0 } }}
        >
          <Tab value="graph" label="Graph" />
          <Tab value="corpus" label="Corpus" />
          <Tab value="log" label="Crawl log" />
          <Tab
            value="scheduler"
            icon={<ScheduleOutlined fontSize="small" />}
            iconPosition="start"
            label="Scheduler"
          />
        </Tabs>

        <ButtonGroup variant="contained">
          <Button
            size="small"
            disabled={isCrawlActive}
            onClick={() => setMenuOpen((open) => !open)}
            endIcon={<ArrowDropDown />}
            aria-haspopup="menu"
            aria-expanded={menuOpen}
            aria-controls={menuOpen ? "project-action-menu" : undefined}
            ref={setActionButton}
          >
            Action
          </Button>
        </ButtonGroup>
        <Popper
          open={menuOpen}
          anchorEl={actionButton}
          role={undefined}
          placement="bottom-end"
          transition
          disablePortal
          sx={{ zIndex: (t) => t.zIndex.modal }}
        >
          {({ TransitionProps }) => (
            <Grow {...TransitionProps}>
              <Paper id="project-action-menu">
                <ClickAwayListener onClickAway={() => setMenuOpen(false)}>
                  <MenuList autoFocusItem>
                    {ACTIONS.map((a) => (
                      <MenuItem
                        key={a.value}
                        disabled={isCrawlActive}
                        onClick={() => {
                          setMenuOpen(false);
                          setTab("graph");
                          void startProjectAction(project.id, a.value);
                        }}
                      >
                        {a.label}
                      </MenuItem>
                    ))}
                  </MenuList>
                </ClickAwayListener>
              </Paper>
            </Grow>
          )}
        </Popper>
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
      {tab === "scheduler" ? (
        <div className="grid grid-cols-1 gap-8 lg:grid-cols-2">
          <SchedulerForm />
          <ActiveCrawlers />
        </div>
      ) : null}
    </SectionShell>
  );
}
