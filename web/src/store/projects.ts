import { create } from "zustand";
import { persist } from "zustand/middleware";

import { MOCK_PROJECTS } from "@/mocks/fixtures/projects";
import type { CrawlLogEntry, Project } from "@/types/domain";

const LOG_SNAPSHOT_CAP = 200;

interface ProjectsState {
  projects: Project[];
  addProject(p: Project): void;
  finalizeProject(
    id: string,
    patch: { status: "complete" | "failed"; docsIngested?: number; logSnapshot?: CrawlLogEntry[] },
  ): void;
}



export const useProjectsStore = create<ProjectsState>()(
  persist(
    (set) => ({
      projects: MOCK_PROJECTS,

      addProject: (p) => set((s) => ({ projects: [p, ...s.projects] })),

      finalizeProject: (id, patch) =>
        set((s) => ({
          projects: s.projects.map((p) =>
            p.id === id
              ? {
                  ...p,
                  status: patch.status,
                  docsIngested: patch.docsIngested ?? p.docsIngested,
                  logSnapshot: (patch.logSnapshot ?? p.logSnapshot).slice(-LOG_SNAPSHOT_CAP),
                }
              : p,
          ),
        })),
    }),
    { name: "reharvester-projects" },
  ),
);
