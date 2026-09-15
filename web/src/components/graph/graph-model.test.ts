/** Run with: bun test src/components/graph/graph-model.test.ts */
import { expect, test } from "bun:test";

import type { GraphEdge, GraphNode, GraphRelation } from "@/types/domain";

import { buildAdjacency, focusRoles, RELATIONS, seedPositions, visibleLinks } from "./graph-model";

const edge = (source: string, target: string, relation: GraphRelation, weight = 1): GraphEdge => ({
  id: `${relation}:${source}->${target}`,
  source,
  target,
  relation,
  weight,
});

const node = (id: string, cluster: string): GraphNode => ({
  id,
  label: id,
  data: {
    kind: "paper",
    docId: id,
    year: 2022,
    pageRank: 0,
    gap: false,
    bridge: 0,
    cluster,
    category: "",
    openAccess: true,
  },
});

const all = new Set(RELATIONS);

const edges = [
  edge("a", "b", "similar-to", 0.4),
  edge("a", "c", "nearest-to", 0.9),
  edge("d", "a", "nearest-to", 0.7),
  edge("concept", "a", "appears-in"),
  edge("a", "d", "co-authored-with"),
  edge("a", "a", "similar-to"),
];

test("mutual relations are listed only under mutual at both ends; directed ones only source→target", () => {
  const adj = buildAdjacency(edges);
  const ids = (links: { other: string }[]) => links.map((l) => l.other);

  expect(ids(adj.get("a")!.out)).toEqual(["c"]);
  expect(ids(adj.get("a")!.in)).toEqual(["concept", "d"]);
  expect(ids(adj.get("a")!.mutual)).toEqual(["d", "b"]);
  expect(ids(adj.get("b")!.out)).toEqual([]);
  expect(ids(adj.get("b")!.in)).toEqual([]);
  expect(ids(adj.get("b")!.mutual)).toEqual(["a"]);
  expect(ids(adj.get("c")!.out)).toEqual([]);
  expect(ids(adj.get("c")!.in)).toEqual(["a"]);
  expect(ids(adj.get("concept")!.out)).toEqual(["a"]);
});

test("In, Out and Mutual modes never leak each other's edges", () => {
  const adj = buildAdjacency(edges);
  const rels = (links: { edge: GraphEdge }[]) => links.map((l) => l.edge.relation);

  expect(rels(visibleLinks(adj, "a", "in", "in", all))).toEqual(["appears-in", "nearest-to"]);
  expect(visibleLinks(adj, "a", "out", "in", all)).toEqual([]);
  expect(visibleLinks(adj, "a", "mutual", "in", all)).toEqual([]);
  expect(rels(visibleLinks(adj, "a", "mutual", "mutual", all))).toEqual(["co-authored-with", "similar-to"]);
  expect(visibleLinks(adj, "a", "in", "mutual", all)).toEqual([]);

  for (const mode of ["in", "out"] as const) {
    const roles = focusRoles(adj, "a", mode, all);
    expect([...roles.edges.values()].every((r) => r === mode)).toBe(true);
    expect([...roles.nodes.values()].every((r) => r === mode)).toBe(true);
  }
  const mutual = focusRoles(adj, "a", "mutual", all);
  expect([...mutual.edges.keys()].sort()).toEqual(["co-authored-with:a->d", "similar-to:a->b"]);
});

test("lists are sorted heaviest first", () => {
  const adj = buildAdjacency(edges);
  const weights = adj.get("a")!.out.map((l) => l.edge.weight);
  expect(weights).toEqual([...weights].sort((x, y) => y - x));
});

test("visibleLinks honours direction mode and relation filter", () => {
  const adj = buildAdjacency(edges);
  expect(visibleLinks(adj, "a", "out", "in", all)).toEqual([]);
  const nearestOnly = new Set<GraphRelation>(["nearest-to"]);
  expect(visibleLinks(adj, "a", "out", "all", nearestOnly).map((l) => l.other)).toEqual(["c"]);
  expect(visibleLinks(adj, "a", "in", "in", nearestOnly).map((l) => l.other)).toEqual(["d"]);
});

test("focusRoles marks direction per edge and merges mixed neighbours to mutual", () => {
  const adj = buildAdjacency(edges);
  const { nodes, edges: roles } = focusRoles(adj, "a", "all", all);

  expect(nodes.get("c")).toBe("out");
  expect(nodes.get("concept")).toBe("in");
  expect(nodes.get("b")).toBe("mutual");
  expect(nodes.get("d")).toBe("mutual");
  expect(roles.get("nearest-to:d->a")).toBe("in");
  expect(roles.get("nearest-to:a->c")).toBe("out");

  const outOnly = focusRoles(adj, "a", "out", new Set<GraphRelation>(["nearest-to"]));
  expect([...outOnly.nodes.keys()]).toEqual(["c"]);
  expect([...outOnly.edges.keys()]).toEqual(["nearest-to:a->c"]);
});

test("seedPositions is order-independent, finite, and keeps communities apart", () => {
  const nodes = [
    ...Array.from({ length: 30 }, (_, i) => node(`x${i}`, "x")),
    ...Array.from({ length: 20 }, (_, i) => node(`y${i}`, "y")),
    ...Array.from({ length: 10 }, (_, i) => node(`z${i}`, "z")),
  ];
  const seeds = seedPositions(nodes);
  const shuffled = seedPositions([...nodes].reverse());
  expect([...shuffled.entries()].sort()).toEqual([...seeds.entries()].sort());

  const centroid = (cluster: string) => {
    const members = nodes.filter((n) => n.data.cluster === cluster).map((n) => seeds.get(n.id)!);
    return {
      x: members.reduce((s, p) => s + p.x, 0) / members.length,
      y: members.reduce((s, p) => s + p.y, 0) / members.length,
    };
  };
  const centroids = { x: centroid("x"), y: centroid("y"), z: centroid("z") };
  for (const n of nodes) {
    const p = seeds.get(n.id)!;
    expect(Number.isFinite(p.x) && Number.isFinite(p.y)).toBe(true);
    const dist = (c: { x: number; y: number }) => Math.hypot(p.x - c.x, p.y - c.y);
    const own = dist(centroids[n.data.cluster as "x" | "y" | "z"]);
    for (const [key, c] of Object.entries(centroids)) {
      if (key !== n.data.cluster) expect(own).toBeLessThan(dist(c));
    }
  }
});
