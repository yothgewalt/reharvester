import type { GraphEdge } from "@/types/domain";

export interface AdjacencyEntry {

  nodes: string[];

  edges: string[];
}


export function buildAdjacencyIndex(edges: GraphEdge[]): Map<string, AdjacencyEntry> {
  const index = new Map<string, AdjacencyEntry>();
  const entry = (id: string): AdjacencyEntry => {
    let e = index.get(id);
    if (!e) {
      e = { nodes: [], edges: [] };
      index.set(id, e);
    }
    return e;
  };
  for (const e of edges) {
    const s = entry(e.source);
    s.nodes.push(e.target);
    s.edges.push(e.id);
    const t = entry(e.target);
    t.nodes.push(e.source);
    t.edges.push(e.id);
  }
  return index;
}
