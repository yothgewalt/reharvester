import type { CrawlLogEntry, CrawlStreamEvent, GraphEdge, GraphNode } from "@/types/domain";

import { DELTA_BATCH } from "./graph";

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
