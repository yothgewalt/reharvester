import type {
  AskRequest,
  AskResponse,
  Community,
  CommunityLink,
  CrawlStreamEvent,
  FieldErrors,
  GapReport,
  GraphSnapshot,
  HarvestInitRequest,
  HarvestInitResponse,
  JobRegisterRequest,
  ProjectActionRequest,
  ProjectActionResponse,
  ProjectSummary,
  SchedulerProfile,
  SettingsPatch,
  SettingsView,
  TrendKeyword,
  WsStatus,
} from "@/types/domain";

export interface AskStreamHandlers {
  onSources(meta: Omit<AskResponse, "answer">): void;
  onToken(text: string): void;
  onDone(answer: string, generated: boolean): void;
  onError(message: string): void;
}

import { createHttpTransport } from "./http-transport";

export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
    public path: string,
    /** Parsed JSON body of a 4xx response, when one was sent. Undefined for 5xx/network errors. */
    public body?: unknown,
  ) {
    super(message);
    this.name = "ApiError";
  }

  /** `{errors}` shape from a 400 (settings PATCH, harvest init); null otherwise. */
  get fieldErrors(): FieldErrors | null {
    if (typeof this.body !== "object" || this.body === null) return null;
    const errors = (this.body as { errors?: unknown }).errors;
    if (typeof errors !== "object" || errors === null) return null;
    const out: FieldErrors = {};
    for (const [key, value] of Object.entries(errors)) {
      if (typeof value === "string") out[key] = value;
    }
    return Object.keys(out).length > 0 ? out : null;
  }
}

export interface CrawlSocketHandle {
  close(): void;
}

export interface ApiTransport {
  harvestInit(req: HarvestInitRequest): Promise<HarvestInitResponse>;
  applyProjectAction(req: ProjectActionRequest): Promise<ProjectActionResponse>;
  registerJob(req: JobRegisterRequest): Promise<SchedulerProfile>;
  getGraphSnapshot(): Promise<GraphSnapshot>;
  getWikiRaw(docId: string): Promise<string>;
  recalculateTrends(window: [number, number]): Promise<TrendKeyword[]>;
  listGapPairs(): Promise<GapReport>;
  listCommunities(): Promise<Community[]>;
  listCommunityLinks(): Promise<CommunityLink[]>;
  ask(req: AskRequest): Promise<AskResponse>;
  /**
   * Streams an answer: sources land in milliseconds, prose arrives token by
   * token over the seconds that follow. Never rejects for a generation
   * failure — that is reported through `onError` so the sources already on
   * screen survive. Returns when the stream closes.
   */
  askStream(req: AskRequest, handlers: AskStreamHandlers, signal?: AbortSignal): Promise<void>;
  /** Model-written keywords for `field`; an empty list means no model answered. */
  randomKeywords(field: string): Promise<string[]>;
  listProjects(): Promise<ProjectSummary[]>;
  listJobs(): Promise<SchedulerProfile[]>;
  getSettings(): Promise<SettingsView>;
  /** Send exactly one changed field; the server rejects the whole patch on any invalid one. */
  patchSettings(patch: SettingsPatch): Promise<SettingsView>;
  openCrawlSocket(
    taskId: string,
    onEvent: (e: CrawlStreamEvent) => void,
    onStatus: (s: WsStatus) => void,
  ): CrawlSocketHandle;
}

export const transport: ApiTransport = createHttpTransport();
