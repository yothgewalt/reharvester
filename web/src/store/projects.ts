import { create } from "zustand";
import { persist } from "zustand/middleware";

import { transport } from "@/lib/api/transport";
import type { CrawlLogEntry, Project } from "@/types/domain";

const LOG_SNAPSHOT_CAP = 200;

interface ProjectsState {
  projects: Project[];
  addProject(p: Project): void;
  finalizeProject(
    id: string,
    patch: { status: "complete" | "failed"; docsIngested?: number; logSnapshot?: CrawlLogEntry[] },
  ): void;
  hydrate(): Promise<void>;
}



export const useProjectsStore = create<ProjectsState>()(
  persist(
    (set) => ({
      projects: [],

      /**
       * Replace the locally-held list with what the backend has on disk, keeping
       * any log snapshots this browser captured during a crawl — the server does
       * not store those.
       */
      hydrate: async () => {
        try {
          const remote = await transport.listProjects();
          set((s) => {
            const localById = new Map(s.projects.map((p) => [p.id, p]));
            return {
              projects: remote.map((p) => ({
                ...p,
                logSnapshot: localById.get(p.id)?.logSnapshot ?? [],
              })),
            };
          });
        } catch {
          // Keep whatever is in localStorage; the header already shows offline.
        }
      },

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
