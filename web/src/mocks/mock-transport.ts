import type { ApiTransport } from "@/lib/api/transport";
import type { HarvestInitRequest, JobRegisterRequest, SchedulerProfile } from "@/types/domain";

import { getGapFixture } from "./fixtures/gaps";
import { getMockSnapshot, MOCK_GRAPH } from "./fixtures/graph";
import { computeTrends } from "./fixtures/trends";
import { generateWikiMarkdown, RICH_DOCS, stubWikiMarkdown } from "./fixtures/wiki";
import { MockCrawlSocket } from "./mock-socket";

const sleep = () => new Promise((r) => setTimeout(r, 150 + Math.random() * 250));


const LLM_STATUS: "ok" | "unreachable" =
  process.env.NEXT_PUBLIC_MOCK_LLM_DOWN === "true" ? "unreachable" : "ok";

let jobSeq = 0;
let taskSeq = 0;
let lastQuery = "";

export function createMockTransport(): ApiTransport {
  return {
    async health() {
      await sleep();
      return { status: "ok", llm: LLM_STATUS, version: "0.1.0-mock" };
    },

    async harvestInit(req: HarvestInitRequest) {
      await sleep();
      lastQuery = req.pdf?.length
        ? `PDFs: ${req.pdf.map((p) => p.name).join(", ")}`
        : req.keywords?.join(", ") ?? req.abstract ?? "";
      const taskId = `mock-task-${++taskSeq}`;
      return { taskId, streamPath: `/api/v1/harvest/stream/${taskId}` };
    },

    async registerJob(req: JobRegisterRequest): Promise<SchedulerProfile> {
      await sleep();
      return {
        id: `job-${++jobSeq}`,
        name: req.name,
        keywords: req.keywords,
        cron: req.cron,
        createdAt: new Date().toISOString(),
        status: "active",
      };
    },

    async getGraphSnapshot(kind) {
      await sleep();
      return getMockSnapshot(kind);
    },

    async getWikiRaw(docId: string) {
      await sleep();
      const rich = RICH_DOCS[docId];
      if (rich) return rich;
      const node = MOCK_GRAPH.nodes.find((n) => n.id === docId);
      if (!node) return stubWikiMarkdown(docId);
      const labelById = new Map(MOCK_GRAPH.nodes.map((n) => [n.id, n.label]));
      const neighbors: Array<{ id: string; label: string }> = [];
      for (const e of MOCK_GRAPH.edges) {
        const otherId = e.source === docId ? e.target : e.target === docId ? e.source : null;
        if (!otherId) continue;
        const label = labelById.get(otherId);
        if (label) neighbors.push({ id: otherId, label });
      }
      return generateWikiMarkdown(node, neighbors);
    },

    async recalculateTrends(window: [number, number]) {
      await sleep();
      return computeTrends(window);
    },

    async getGapPositions() {
      await sleep();
      return getGapFixture();
    },

    openCrawlSocket(taskId, onEvent, onStatus) {
      void taskId;
      return new MockCrawlSocket(lastQuery, onEvent, onStatus);
    },
  };
}
