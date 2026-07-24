import { createMockTransport } from "@/mocks/mock-transport";
import { USE_MOCKS } from "@/lib/config";
import type {
  CrawlStreamEvent,
  GapPositionsResponse,
  GraphKind,
  GraphSnapshot,
  HarvestInitRequest,
  HarvestInitResponse,
  HealthStatus,
  JobRegisterRequest,
  ProjectAction,
  ProjectActionRequest,
  ProjectActionResponse,
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
  health(): Promise<HealthStatus>;
  harvestInit(req: HarvestInitRequest): Promise<HarvestInitResponse>;
  applyProjectAction(req: ProjectActionRequest): Promise<ProjectActionResponse>;
  registerJob(req: JobRegisterRequest): Promise<SchedulerProfile>;
  getGraphSnapshot(kind: GraphKind): Promise<GraphSnapshot>;
  getWikiRaw(docId: string): Promise<string>;
  recalculateTrends(window: [number, number]): Promise<TrendKeyword[]>;
  getGapPositions(): Promise<GapPositionsResponse>;
  openCrawlSocket(
    taskId: string,
    onEvent: (e: CrawlStreamEvent) => void,
    onStatus: (s: WsStatus) => void,
    action?: ProjectAction,
  ): CrawlSocketHandle;
}

export const transport: ApiTransport = USE_MOCKS ? createMockTransport() : createHttpTransport();
