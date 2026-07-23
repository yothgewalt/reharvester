import type { Project } from "@/types/domain";

const now = new Date().toISOString();
const yesterday = new Date(Date.now() - 86_400_000).toISOString();


export const MOCK_PROJECTS: Project[] = [
  {
    id: "mock-project-transformers",
    name: "Transformer Architectures · Attention",
    query: "keywords: transformer architectures, attention mechanisms, scaling laws",
    createdAt: yesterday,
    status: "complete",
    docsIngested: 124,
    logSnapshot: [
      {
        id: "mock-log-1",
        timestamp: yesterday,
        level: "info",
        message: "Harvest task accepted: \"transformer architectures, attention mechanisms, scaling laws\"",
      },
      {
        id: "mock-log-2",
        timestamp: yesterday,
        level: "success",
        message: "Harvest complete — 124 documents ingested",
      },
    ],
  },
  {
    id: "mock-project-rag",
    name: "Retrieval-Augmented Generation · Dense Passage",
    query: "keywords: retrieval-augmented generation, vector databases, dense passage retrieval",
    createdAt: now,
    status: "crawling",
    docsIngested: 0,
    logSnapshot: [
      {
        id: "mock-log-3",
        timestamp: now,
        level: "info",
        message: "Harvest task accepted: \"retrieval-augmented generation, vector databases, dense passage retrieval\"",
      },
    ],
  },
];
