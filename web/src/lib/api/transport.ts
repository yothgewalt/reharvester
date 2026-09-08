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
  listProjects(): Promise<ProjectSummary[]>;
  listJobs(): Promise<SchedulerProfile[]>;
  openCrawlSocket(
    taskId: string,
    onEvent: (e: CrawlStreamEvent) => void,
    onStatus: (s: WsStatus) => void,
  ): CrawlSocketHandle;
}

export const transport: ApiTransport = createHttpTransport();
