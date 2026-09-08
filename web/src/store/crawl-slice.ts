import type { StateCreator } from "zustand";

import { transport, type CrawlSocketHandle } from "@/lib/api/transport";
import type {
  CrawlLogEntry,
  CrawlStreamEvent,
  IngestPayload,
  ProjectAction,
  WsStatus,
} from "@/types/domain";

import type { AppState } from "./index";
import { useProjectsStore } from "./projects";

const projectName = (payload: IngestPayload): string => {
  if (payload.mode === "keywords") return payload.keywords.slice(0, 3).join(" · ");
  if (payload.mode === "abstract") return `${payload.abstract.split(/\s+/).slice(0, 6).join(" ")}…`;
  const first = payload.files[0]?.name.replace(/\.pdf$/i, "") ?? "PDF upload";
  return payload.files.length > 1 ? `${first} +${payload.files.length - 1}` : first;
};

const fileToBase64 = (file: File): Promise<string> =>
  new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => {
      const data = reader.result as string;
      const base64 = data.split(",")[1] ?? "";
      resolve(base64);
    };
    reader.onerror = reject;
    reader.readAsDataURL(file);
  });

const LOG_CAP = 2000;


let socketHandle: CrawlSocketHandle | null = null;
let localLogSeq = 0;

const localLog = (level: CrawlLogEntry["level"], message: string): CrawlLogEntry => ({
  id: `local-${++localLogSeq}`,
  timestamp: new Date().toISOString(),
  level,
  message,
});

export interface CrawlSlice {
  isCrawlActive: boolean;
  activeTask: string | null;
  activeTaskId: string | null;
  crawlProgress: number | null;
  crawlStage: string | null;
  consoleLogHistory: CrawlLogEntry[];
  wsStatus: WsStatus;
  startCrawl(payload: IngestPayload): Promise<void>;
  startProjectAction(projectId: string, action: ProjectAction): Promise<void>;
  appendLog(entry: CrawlLogEntry): void;
  handleStreamEvent(e: CrawlStreamEvent): void;
  finishCrawl(): void;
}

export const createCrawlSlice: StateCreator<AppState, [], [], CrawlSlice> = (set, get) => ({
  isCrawlActive: false,
  activeTask: null,
  activeTaskId: null,
  crawlProgress: null,
  crawlStage: null,
  consoleLogHistory: [],
  wsStatus: "idle",

  startCrawl: async (payload) => {
    if (get().isCrawlActive) return;
    const summary =
      payload.mode === "keywords"
        ? payload.keywords.join(", ")
        : payload.mode === "abstract"
          ? `${payload.abstract.split(/\s+/).filter(Boolean).length}-word abstract`
          : `${payload.files.length} PDF${payload.files.length === 1 ? "" : "s"}: ${payload.files
              .map((f) => f.name)
              .join(", ")}`;
    set({
      isCrawlActive: true,
      activeTask: summary,
      activeTaskId: null,
      crawlProgress: null,
      crawlStage: null,
      consoleLogHistory: [],
      wsStatus: "connecting",
    });
    get().appendLog(localLog("info", "Submitting harvest request …"));


    if (get().nodes.length === 0 && !get().graphLoading) void get().loadGraphSnapshot();
    try {
      const req: { keywords?: string[]; abstract?: string; pdf?: Array<{ name: string; base64: string }> } =
        payload.mode === "keywords"
          ? { keywords: payload.keywords }
          : payload.mode === "abstract"
            ? { abstract: payload.abstract }
            : {
                pdf: await Promise.all(
                  payload.files.map(async (file) => ({
                    name: file.name,
                    base64: await fileToBase64(file),
                  })),
                ),
              };
      const res = await transport.harvestInit(req);
      set({ activeTaskId: res.taskId });
      useProjectsStore.getState().addProject({
        id: res.taskId,
        name: projectName(payload),
        query: summary,
        createdAt: new Date().toISOString(),
        status: "crawling",
        docsIngested: 0,
        logSnapshot: [],
      });
      socketHandle = transport.openCrawlSocket(
        res.taskId,
        (e) => get().handleStreamEvent(e),
        (s: WsStatus) => {
          set({ wsStatus: s });

          if (s === "closed" && get().isCrawlActive) get().finishCrawl();
        },
      );
    } catch (err) {
      get().appendLog(
        localLog("error", `Harvest init failed: ${err instanceof Error ? err.message : String(err)}`),
      );
      set({ isCrawlActive: false, wsStatus: "closed" });
    }
  },

  startProjectAction: async (projectId, action) => {
    if (get().isCrawlActive) return;
    const project = useProjectsStore.getState().projects.find((p) => p.id === projectId);
    if (!project) return;

    set({
      isCrawlActive: true,
      activeTask: `${project.name} — ${action}`,
      activeTaskId: projectId,
      crawlProgress: null,
      crawlStage: null,
      consoleLogHistory: [],
      wsStatus: "connecting",
    });
    get().appendLog(localLog("info", `Starting project action: ${action} …`));

    if (get().nodes.length === 0 && !get().graphLoading) void get().loadGraphSnapshot();
    try {
      const res = await transport.applyProjectAction({ projectId, action });
      set({ activeTaskId: res.taskId });
      socketHandle = transport.openCrawlSocket(
        res.taskId,
        (e) => get().handleStreamEvent(e),
        (s: WsStatus) => {
          set({ wsStatus: s });
          if (s === "closed" && get().isCrawlActive) get().finishCrawl();
        },
      );
    } catch (err) {
      get().appendLog(
        localLog("error", `Project action failed: ${err instanceof Error ? err.message : String(err)}`),
      );
      set({ isCrawlActive: false, wsStatus: "closed" });
    }
  },

  appendLog: (entry) =>
    set((s) => ({
      consoleLogHistory:
        s.consoleLogHistory.length >= LOG_CAP
          ? [...s.consoleLogHistory.slice(-(LOG_CAP - 1)), entry]
          : [...s.consoleLogHistory, entry],
    })),

  handleStreamEvent: (e) => {
    switch (e.type) {
      case "log":
        get().appendLog(e.entry);
        break;
      case "progress":
        set({ crawlProgress: e.percent, crawlStage: e.stage });
        break;
      case "graph_delta":
        get().applyGraphDelta({ addedNodes: e.addedNodes, addedEdges: e.addedEdges });
        break;
      case "done": {
        set({ crawlProgress: 100, crawlStage: "done" });
        const { activeTaskId, consoleLogHistory } = get();
        if (activeTaskId) {
          useProjectsStore.getState().finalizeProject(activeTaskId, {
            status: "complete",
            docsIngested: e.summary.docsIngested,
            logSnapshot: consoleLogHistory,
          });
        }
        get().finishCrawl();
        break;
      }
    }
  },

  finishCrawl: () => {


    const handle = socketHandle;
    socketHandle = null;
    const { activeTaskId, consoleLogHistory } = get();
    set({ isCrawlActive: false, wsStatus: "closed" });
    handle?.close();


    if (activeTaskId) {
      const project = useProjectsStore.getState().projects.find((p) => p.id === activeTaskId);
      if (project?.status === "crawling") {
        useProjectsStore.getState().finalizeProject(activeTaskId, {
          status: "failed",
          logSnapshot: consoleLogHistory,
        });
      }
    }
  },
});
