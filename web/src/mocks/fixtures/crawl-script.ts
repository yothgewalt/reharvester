import type { CrawlLogEntry, CrawlStreamEvent, GraphEdge, GraphNode, ProjectAction } from "@/types/domain";

import { DELTA_BATCH, generateDeltaBatch } from "./graph";

let logSeq = 0;
const log = (level: CrawlLogEntry["level"], message: string): CrawlStreamEvent => ({
  type: "log",
  entry: {
    id: `mock-log-${++logSeq}`,
    timestamp: new Date().toISOString(),
    level,
    message,
  },
});

const progress = (percent: number, stage: string): CrawlStreamEvent => ({
  type: "progress",
  percent,
  stage,
});

const delta = (nodes: GraphNode[], edges: GraphEdge[]): CrawlStreamEvent => ({
  type: "graph_delta",
  addedNodes: nodes,
  addedEdges: edges,
});

export interface ScriptStep {
  delayMs: number;
  event: CrawlStreamEvent;
}

const actionMessages: Record<ProjectAction, { started: string; done: string }> = {
  add: { started: "Expanding graph with additional documents …", done: "Documents added to project graph" },
  subtract: { started: "Removing unrelated nodes from project graph …", done: "Graph pruned by project context" },
  extract: { started: "Extracting entities and relations from project docs …", done: "Entities extracted and linked" },
  summarize: { started: "Summarizing project cluster summaries …", done: "Summaries folded into graph" },
};

export function buildCrawlScript(query: string): ScriptStep[] {
  const chunks: Array<{ nodes: GraphNode[]; edges: GraphEdge[] }> = [];
  const chunkSize = 5;
  for (let i = 0; i < DELTA_BATCH.nodes.length; i += chunkSize) {
    const nodes = DELTA_BATCH.nodes.slice(i, i + chunkSize);
    const ids = new Set(nodes.map((n) => n.id));
    chunks.push({ nodes, edges: DELTA_BATCH.edges.filter((e) => ids.has(e.source)) });
  }

  const steps: ScriptStep[] = [
    { delayMs: 200, event: log("info", `Harvest task accepted: "${query.slice(0, 80)}"`) },
    { delayMs: 500, event: log("info", "Resolving local corpus index (SQLite) …") },
    { delayMs: 700, event: log("info", "Connecting to OpenAlex works endpoint …") },
    { delayMs: 600, event: progress(4, "query-expansion") },
    { delayMs: 500, event: log("info", "Expanding query terms → 7 variants") },
    { delayMs: 900, event: log("info", "Fetching page 1/6 (200 records)") },
    { delayMs: 700, event: progress(12, "retrieval") },
  ];

  let pct = 16;
  chunks.forEach((chunk, i) => {
    steps.push(
      { delayMs: 900, event: log("info", `Parsing citation paths for batch ${i + 1}/${chunks.length}`) },
      { delayMs: 600, event: progress(Math.min(92, pct), "citation-graph") },
      {
        delayMs: 500,
        event: log("success", `Registered ${chunk.nodes.length} new work item(s)`),
      },
      { delayMs: 400, event: delta(chunk.nodes, chunk.edges) },
    );
    if (i === 2) {
      steps.push({
        delayMs: 600,
        event: log("warn", "Rate limited by upstream API — backing off 1.2s"),
      });
    }
    pct += Math.floor(76 / chunks.length);
  });

  steps.push(
    { delayMs: 800, event: progress(95, "wiki-synthesis") },
    { delayMs: 900, event: log("info", "Synthesizing wiki stubs for harvested items …") },
    { delayMs: 900, event: log("info", "Recomputing PageRank over merged graph") },
    { delayMs: 700, event: progress(100, "done") },
    { delayMs: 400, event: log("success", `Harvest complete — ${DELTA_BATCH.nodes.length} documents ingested`) },
    {
      delayMs: 300,
      event: {
        type: "done",
        summary: { docsIngested: DELTA_BATCH.nodes.length, durationMs: 30_000 },
      },
    },
  );

  return steps;
}

export function buildActionScript(action: ProjectAction, query: string): ScriptStep[] {
  const batch = generateDeltaBatch(Date.now());
  const chunkSize = 5;
  const chunks: Array<{ nodes: GraphNode[]; edges: GraphEdge[] }> = [];
  for (let i = 0; i < batch.nodes.length; i += chunkSize) {
    const nodes = batch.nodes.slice(i, i + chunkSize);
    const ids = new Set(nodes.map((n) => n.id));
    chunks.push({ nodes, edges: batch.edges.filter((e) => ids.has(e.source)) });
  }

  const messages = actionMessages[action];
  const steps: ScriptStep[] = [
    { delayMs: 200, event: log("info", `Action "${action}" requested for: "${query.slice(0, 80)}"`) },
    { delayMs: 400, event: log("info", messages.started) },
    { delayMs: 500, event: progress(15, `${action}-start`) },
  ];

  let pct = 25;
  chunks.forEach((chunk, i) => {
    steps.push(
      { delayMs: 600, event: log("info", `Processing batch ${i + 1}/${chunks.length}`) },
      { delayMs: 400, event: progress(Math.min(90, pct), `${action}-batch`) },
      { delayMs: 400, event: delta(chunk.nodes, chunk.edges) },
    );
    pct += Math.floor(60 / chunks.length);
  });

  steps.push(
    { delayMs: 500, event: progress(100, "done") },
    { delayMs: 300, event: log("success", messages.done) },
    {
      delayMs: 200,
      event: { type: "done", summary: { docsIngested: batch.nodes.length, durationMs: 8_000 } },
    },
  );

  return steps;
}
