import type {
  AskRequest,
  AskResponse,
  Community,
  CommunityLink,
  CrawlStreamEvent,
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
  ) {
    super(message);
    this.name = "ApiError";
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
  listProjects(): Promise<ProjectSummary[]>;
  listJobs(): Promise<SchedulerProfile[]>;
  openCrawlSocket(
    taskId: string,
    onEvent: (e: CrawlStreamEvent) => void,
    onStatus: (s: WsStatus) => void,
  ): CrawlSocketHandle;
}

export const transport: ApiTransport = createHttpTransport();
