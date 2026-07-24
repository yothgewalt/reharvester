import { API_BASE } from "@/lib/config";
import type {
  GapPositionsResponse,
  GraphSnapshot,
  HarvestInitRequest,
  HarvestInitResponse,
  HealthStatus,
  JobRegisterRequest,
  ProjectActionRequest,
  ProjectActionResponse,
  SchedulerProfile,
  TrendKeyword,
} from "@/types/domain";

import { openReconnectingCrawlSocket } from "./crawl-socket";
import { reportBackendFailure } from "./failure-bus";
import { ApiError, type ApiTransport } from "./transport";

const TIMEOUT_MS = 10_000;

async function request(path: string, init?: RequestInit): Promise<Response> {
  let res: Response;
  try {
    res = await fetch(`${API_BASE}${path}`, {
      ...init,
      signal: AbortSignal.timeout(TIMEOUT_MS),
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

async function postJson<T>(path: string, body: unknown): Promise<T> {
  const res = await request(path, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  return (await res.json()) as T;
}

export function createHttpTransport(): ApiTransport {
  return {
    health: () => getJson<HealthStatus>("/health"),
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
    getGraphSnapshot: (kind) => getJson<GraphSnapshot>(`/api/v1/graph/snapshot?kind=${kind}`),
    getWikiRaw: async (docId: string) => {
      const res = await request(`/api/v1/wiki/raw/${encodeURIComponent(docId)}`);
      return res.text();
    },
    recalculateTrends: ([start, end]: [number, number]) =>
      getJson<TrendKeyword[]>(`/api/v1/trends/recalculate?start=${start}&end=${end}`),
    getGapPositions: () => getJson<GapPositionsResponse>("/api/v1/gaps/positions"),
    openCrawlSocket: (taskId, onEvent, onStatus, _action) =>
      openReconnectingCrawlSocket(taskId, onEvent, onStatus),
  };
}
