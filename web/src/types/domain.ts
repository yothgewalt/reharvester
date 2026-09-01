




export interface GraphNodeData {
  kind: "paper" | "concept";
  docId: string;
  year: number;
  pageRank: number;
  gap: boolean;
  bridge: number;
  cluster: string;
  category: string;
  openAccess: boolean;
  frequency?: number;
}

export interface GraphNode {
  id: string;
  label: string;
  data: GraphNodeData;
}

/** Emitted by internal/httpapi/snapshot.go. "similar-to" carries the backbone
 *  cosine in `weight`; the others are structural and always weight 1. */
export type GraphRelation = "similar-to" | "co-authored-with" | "appears-in";

export interface GraphEdge {
  id: string;
  source: string;
  target: string;
  weight: number;

  relation: GraphRelation;
}

export interface GraphSnapshot {
  nodes: GraphNode[];
  edges: GraphEdge[];
  generatedAt: string;
}

/** Two communities that read alike but are under-connected — paper Fig. 2d. */
export interface GapPair {
  a: number;
  b: number;
  labelA: string;
  labelB: string;
  similarity: number;
  observed: number;
  nullMean: number;
  nullStd: number;
  z: number;
}

/** `belowThreshold` of `total` pairs clear z < -2. When nearly all of them do,
 *  the z-score is a weak filter and centroid similarity carries the ranking —
 *  the report ships both so the UI can say so. */
export interface GapReport {
  pairs: GapPair[];
  belowThreshold: number;
  total: number;
}

/** One Louvain partition. `size` counts every member; `memberDocIds` lists only
 *  those inside the graph snapshot, so it is a subset. */
export interface Community {
  id: number;
  label: string;
  slug: string;
  size: number;
  terms: string[];
  memberDocIds: string[];
}

/** One edge of the coarse-grained field map; source/target are Community.id. */
export interface CommunityLink {
  source: number;
  target: number;
  count: number;
}

export interface AskRequest {
  question: string;
  hops?: number;
  budget?: number;
}

/** `docId` is "" when the paper falls outside the snapshot — render it inert. */
export interface AskSource {
  docId: string;
  title: string;
  viaGraph: boolean;
  words: number;
}

export interface AskResponse {
  answer: string;
  sources: AskSource[];
  tier: CapabilityTier;
  budget: number;
  hops: number;
}

export type TrendDirection = "rising" | "declining";

export interface TrendKeyword {
  keyword: string;
  /** Late-window document count; `earlyDocs` is the early-window one. A lift is
   *  a share comparison, so on unequal windows raw counts can move the other way. */
  count: number;
  earlyDocs: number;
  direction: TrendDirection;
  /** Normalised entropy over communities that admitted the term (<= 0.80). */
  specificity: number;
  /** Kleinberg burst weight: high when the rise is concentrated in time. */
  burst: number;
  /** Smoothed log2 prevalence lift of the selected window against the one before it. */
  growthRate: number;
  rank: number;
  /**
   * Share of documents mentioning the term, per year, across the whole corpus.
   * Net growth hides substitution — a phrase that peaked and collapsed can carry
   * the same lift as one still climbing.
   */
  trajectory?: TrendPoint[];
}

export interface TrendPoint {
  year: number;
  prevalence: number;
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

/**
 * What GET /api/v1/projects returns. The backend owns everything here except
 * logSnapshot, which only ever exists in the browser that watched the crawl.
 */
export type ProjectSummary = Omit<Project, "logSnapshot">;

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
  /** Rung of the capability ladder the backend is actually running at. */
  tier?: CapabilityTier;
}

/**
 * T0 inverted index only · T1 + TF-IDF · T2 + k-NN backbone · T3 + encoder.
 * Each tier is a complete system; the ladder is a configuration, not a fallback
 * path taken on error.
 */
export type CapabilityTier = "T0" | "T1" | "T2" | "T3";

export type WsStatus = "idle" | "connecting" | "open" | "reconnecting" | "closed";
