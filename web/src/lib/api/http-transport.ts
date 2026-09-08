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


/**
 * Reads one Server-Sent Events stream from a POST.
 *
 * EventSource cannot POST, so this drives fetch's ReadableStream directly.
 * Frames are separated by a blank line; anything not yet terminated stays in
 * the buffer until the rest of it arrives, because a chunk boundary can fall
 * anywhere — including mid-frame, which is exactly what token-by-token output
 * produces.
 */
async function readSSE(
  res: Response,
  onEvent: (event: string, data: string) => void,
): Promise<void> {
  const body = res.body;
  if (!body) throw new Error("no response body to stream");
  const reader = body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";

  for (;;) {
    const { done, value } = await reader.read();
    if (done) break;
    buffer += decoder.decode(value, { stream: true });

    let split = buffer.indexOf("\n\n");
    while (split !== -1) {
      const frame = buffer.slice(0, split);
      buffer = buffer.slice(split + 2);
      let event = "message";
      const data: string[] = [];
      for (const line of frame.split("\n")) {
        if (line.startsWith("event:")) event = line.slice(6).trim();
        else if (line.startsWith("data:")) data.push(line.slice(5).trim());
      }
      if (data.length > 0) onEvent(event, data.join("\n"));
      split = buffer.indexOf("\n\n");
    }
  }
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

    askStream: async (req, handlers, signal) => {
      // No AbortSignal.timeout here: the stream is alive as long as tokens keep
      // arriving, and generation legitimately runs for minutes on a slow CPU.
      // The caller aborts instead, which is what a new question does.
      let res: Response;
      try {
        res = await fetch(`${API_BASE}/api/v1/ask/stream`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(req),
          signal,
        });
      } catch (err) {
        if ((err as Error)?.name === "AbortError") return;
        reportBackendFailure();
        throw new ApiError(0, "cannot reach the local API", "/api/v1/ask/stream");
      }
      if (!res.ok) {
        throw new ApiError(res.status, await res.text().catch(() => res.statusText), "/api/v1/ask/stream");
      }
      try {
        await readSSE(res, (event, data) => {
          switch (event) {
            case "sources":
              handlers.onSources(JSON.parse(data));
              break;
            case "token":
              handlers.onToken(JSON.parse(data).text as string);
              break;
            case "done": {
              const d = JSON.parse(data);
              handlers.onDone(d.answer as string, Boolean(d.generated));
              break;
            }
            case "error":
              handlers.onError(JSON.parse(data).message as string);
              break;
          }
        });
      } catch (err) {
        if ((err as Error)?.name === "AbortError") return;
        throw err;
      }
    },
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
