

export type LayoutPreset =
  | "forceDirected2d"
  | "forceDirected3d"
  | "concentric2d"
  | "concentric3d"
  | "circular2d";


export type GraphKind = "knowledge" | "cooccurrence";

export interface GraphNodeData {
  kind: "paper" | "concept";
  docId: string;
  year: number;
  pageRank: number;
  gap: boolean;
  cluster: string;
  openAccess: boolean;
  frequency?: number;
}

export interface GraphNode {
  id: string;
  label: string;
  data: GraphNodeData;
}

export interface GraphEdge {
  id: string;
  source: string;
  target: string;
  weight: number;

  relation: string;
}

export interface GraphSnapshot {
  nodes: GraphNode[];
  edges: GraphEdge[];
  generatedAt: string;
}

export interface TrendKeyword {
  keyword: string;
  count: number;
  growthRate: number;
  rank: number;
}

export interface WikiDoc {
  id: string;
  title: string;
  markdown: string;
}

export interface Project {
  id: string;
  name: string;
  query: string;
  createdAt: string;
  status: "crawling" | "complete" | "failed";
  docsIngested: number;
  logSnapshot: CrawlLogEntry[];
}

export type ProjectAction = "add" | "subtract" | "extract" | "summarize";

export interface ProjectActionRequest {
  projectId: string;
  action: ProjectAction;
}

export interface ProjectActionResponse {
  taskId: string;
  streamPath: string;
}

export interface SchedulerProfile {
  id: string;
  name: string;
  keywords: string[];
  cron: string;
  createdAt: string;
  status: "active" | "paused";
}

export interface CrawlLogEntry {
  id: string;
  timestamp: string;
  level: "info" | "warn" | "error" | "success";
  message: string;
}

export type CrawlStreamEvent =
  | { type: "log"; entry: CrawlLogEntry }
  | { type: "progress"; percent: number; stage: string }
  | { type: "graph_delta"; addedNodes: GraphNode[]; addedEdges: GraphEdge[] }
  | { type: "done"; summary: { docsIngested: number; durationMs: number } };

export type IngestPayload =
  | { mode: "keywords"; keywords: string[] }
  | { mode: "abstract"; abstract: string }
  | { mode: "pdf"; files: File[] };

export interface HarvestInitRequest {
  keywords?: string[];
  abstract?: string;
  pdf?: Array<{ name: string; base64: string }>;
}
export interface HarvestInitResponse {
  taskId: string;
  streamPath: string;
}

export interface JobRegisterRequest {
  name: string;
  keywords: string[];
  cron: string;
}

export interface HealthStatus {
  status: "ok";
  llm: "ok" | "unreachable";
  version: string;
}

export interface GapPositionsResponse {
  gapNodeIds: string[];
}

export type WsStatus = "idle" | "connecting" | "open" | "reconnecting" | "closed";
