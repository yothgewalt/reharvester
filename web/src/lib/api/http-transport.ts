import { API_BASE } from "@/lib/config";
import type {
  AskRequest,
  AskResponse,
  Community,
  CommunityLink,
  GapReport,
  GraphSnapshot,
  HarvestInitRequest,
  HarvestInitResponse,
  JobRegisterRequest,
  ProjectActionRequest,
  ProjectActionResponse,
  ProjectSummary,
  SchedulerProfile,
  TrendKeyword,
} from "@/types/domain";

import { openReconnectingCrawlSocket } from "./crawl-socket";
import { reportBackendFailure } from "./failure-bus";
import { ApiError, type ApiTransport } from "./transport";

const TIMEOUT_MS = 10_000;
// Generation is allowed 60 s server-side (internal/httpapi/server.go), so an ask
// on the default budget would otherwise abort and report the backend as down.
const ASK_TIMEOUT_MS = 65_000;

async function request(path: string, init?: RequestInit, timeoutMs = TIMEOUT_MS): Promise<Response> {
  let res: Response;
  try {
    res = await fetch(`${API_BASE}${path}`, {
      ...init,
      signal: AbortSignal.timeout(timeoutMs),
    });
  } catch (err) {
    reportBackendFailure();
    throw new ApiError(0, err instanceof Error ? err.message : "Network error", path);
  }
  if (!res.ok) {
    if (res.status >= 500) reportBackendFailure();
    throw new ApiError(res.status, `${res.status} ${res.statusText}`, path);
  }
  return res;
}

async function getJson<T>(path: string): Promise<T> {
  const res = await request(path);
  return (await res.json()) as T;
}

async function postJson<T>(path: string, body: unknown, timeoutMs?: number): Promise<T> {
  const res = await request(
    path,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    },
    timeoutMs,
  );
  return (await res.json()) as T;
}

export function createHttpTransport(): ApiTransport {
  return {
    listCommunities: () => getJson<Community[]>("/api/v1/communities"),
    listGapPairs: () => getJson<GapReport>("/api/v1/gaps/pairs"),
    listCommunityLinks: () => getJson<CommunityLink[]>("/api/v1/communities/links"),
    ask: (req: AskRequest) => postJson<AskResponse>("/api/v1/ask", req, ASK_TIMEOUT_MS),
    harvestInit: (req: HarvestInitRequest) => {
      if (req.pdf && req.pdf.length > 0) {

        const body = new FormData();
        for (const entry of req.pdf) {
          body.append(
            "pdf",
            new File([Uint8Array.from(atob(entry.base64), (c) => c.charCodeAt(0))], entry.name),
          );
        }
        return request("/api/v1/harvest/init", { method: "POST", body }).then(
          async (res) => (await res.json()) as HarvestInitResponse,
        );
      }
      return postJson<HarvestInitResponse>("/api/v1/harvest/init", req);
    },
    registerJob: (req: JobRegisterRequest) =>
      postJson<SchedulerProfile>("/api/v1/jobs/register", req),
    applyProjectAction: ({ projectId, action }: ProjectActionRequest) =>
      postJson<ProjectActionResponse>(`/api/v1/projects/${encodeURIComponent(projectId)}/action`, { action }),
    getGraphSnapshot: () => getJson<GraphSnapshot>("/api/v1/graph/snapshot"),
    getWikiRaw: async (docId: string) => {
      const res = await request(`/api/v1/wiki/raw/${encodeURIComponent(docId)}`);
      return res.text();
    },
    recalculateTrends: ([start, end]: [number, number]) =>
      getJson<TrendKeyword[]>(`/api/v1/trends/recalculate?start=${start}&end=${end}`),
    listProjects: () => getJson<ProjectSummary[]>("/api/v1/projects"),
    listJobs: () => getJson<SchedulerProfile[]>("/api/v1/jobs"),
    openCrawlSocket: (taskId, onEvent, onStatus) =>
      openReconnectingCrawlSocket(taskId, onEvent, onStatus),
  };
}
