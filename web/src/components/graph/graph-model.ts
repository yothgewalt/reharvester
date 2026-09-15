import type { GraphEdge, GraphNode, GraphRelation } from "@/types/domain";

export type Direction = "all" | "out" | "in";
export type Role = "out" | "in" | "mutual";

export const RELATIONS: readonly GraphRelation[] = [
  "similar-to",
  "nearest-to",
  "co-authored-with",
  "appears-in",
];

export const RELATION_LABEL: Record<GraphRelation, string> = {
  "similar-to": "Similar to",
  "nearest-to": "Nearest to",
  "co-authored-with": "Co-authored with",
  "appears-in": "Appears in",
};

/** Symmetric relations: listed under both Out and In at both endpoints. */
export const MUTUAL: ReadonlySet<GraphRelation> = new Set(["similar-to", "co-authored-with"]);

export interface Link {
  edge: GraphEdge;
  other: string;
}

export interface Adjacency {
  out: Link[];
  in: Link[];
}

/** Lists are sorted by weight, heaviest first. Build once per edges array. */
export function buildAdjacency(edges: readonly GraphEdge[]): Map<string, Adjacency> {
  const adj = new Map<string, Adjacency>();
  const at = (id: string) => {
    let a = adj.get(id);
    if (!a) {
      a = { out: [], in: [] };
      adj.set(id, a);
    }
    return a;
  };
  for (const edge of edges) {
    if (edge.source === edge.target) continue;
    at(edge.source).out.push({ edge, other: edge.target });
    at(edge.target).in.push({ edge, other: edge.source });
    if (MUTUAL.has(edge.relation)) {
      at(edge.source).in.push({ edge, other: edge.target });
      at(edge.target).out.push({ edge, other: edge.source });
    }
  }
  for (const a of adj.values()) {
    a.out.sort(byWeight);
    a.in.sort(byWeight);
  }
  return adj;
}

function byWeight(a: Link, b: Link): number {
  return b.edge.weight - a.edge.weight || (a.other < b.other ? -1 : a.other > b.other ? 1 : 0);
}

export function visibleLinks(
  adj: ReadonlyMap<string, Adjacency>,
  id: string,
  dir: "out" | "in",
  mode: Direction,
  relations: ReadonlySet<GraphRelation>,
): Link[] {
  if (mode !== "all" && mode !== dir) return [];
  return (adj.get(id)?.[dir] ?? []).filter((l) => relations.has(l.edge.relation));
}

export interface FocusRoles {
  nodes: Map<string, Role>;
  edges: Map<string, Role>;
}

/** Neighbours and edges of `id` under the current filters. A neighbour reached
 *  both ways (e.g. nearest-to plus co-authored-with) is "mutual". */
export function focusRoles(
  adj: ReadonlyMap<string, Adjacency>,
  id: string,
  mode: Direction,
  relations: ReadonlySet<GraphRelation>,
): FocusRoles {
  const nodes = new Map<string, Role>();
  const edges = new Map<string, Role>();
  for (const dir of ["out", "in"] as const) {
    for (const { edge, other } of visibleLinks(adj, id, dir, mode, relations)) {
      const role: Role = MUTUAL.has(edge.relation) ? "mutual" : dir;
      edges.set(edge.id, role);
      const prev = nodes.get(other);
      nodes.set(other, prev && prev !== role ? "mutual" : role);
    }
  }
  return { nodes, edges };
}

const GOLDEN_ANGLE = Math.PI * (3 - Math.sqrt(5));

/** Deterministic starting layout: one disc per community on a sunflower
 *  spiral, independent of node order. Feed it to ForceAtlas2 as x/y. */
export function seedPositions(nodes: readonly GraphNode[]): Map<string, { x: number; y: number }> {
  const clusters = new Map<string, string[]>();
  for (const n of nodes) {
    const key = n.data.cluster || "unclustered";
    const ids = clusters.get(key);
    if (ids) ids.push(n.id);
    else clusters.set(key, [n.id]);
  }
  const ordered = [...clusters.entries()].sort(
    (a, b) => b[1].length - a[1].length || (a[0] < b[0] ? -1 : 1),
  );
  const spread = 5 * Math.sqrt(ordered[0]?.[1].length ?? 1);

  const out = new Map<string, { x: number; y: number }>();
  ordered.forEach(([, ids], i) => {
    const r = spread * Math.sqrt(i);
    const cx = r * Math.cos(i * GOLDEN_ANGLE);
    const cy = r * Math.sin(i * GOLDEN_ANGLE);
    const radius = 2 * Math.sqrt(ids.length);
    for (const id of ids) {
      const h = fnv1a(id);
      const angle = ((h & 0xffff) / 0x10000) * 2 * Math.PI;
      const dist = radius * Math.sqrt(((h >>> 16) & 0xffff) / 0x10000);
      out.set(id, { x: cx + dist * Math.cos(angle), y: cy + dist * Math.sin(angle) });
    }
  });
  return out;
}

function fnv1a(s: string): number {
  let h = 0x811c9dc5;
  for (let i = 0; i < s.length; i++) {
    h ^= s.charCodeAt(i);
    h = Math.imul(h, 0x01000193);
  }
  return h >>> 0;
}
