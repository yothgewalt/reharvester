/** Run with: bun test src/components/trends/field-map-layout.test.ts */
import { expect, test } from "bun:test";

import type { Community, CommunityLink } from "@/types/domain";

import { buildFieldMap, displayLabel, MIN_COMMUNITY_SIZE } from "./field-map-layout";

const community = (id: number, size: number, terms: string[]): Community => ({
  id,
  label: terms.slice(0, 3).join(" "),
  slug: terms.slice(0, 3).join("-"),
  size,
  terms,
  memberDocIds: [],
});

const corpus: Community[] = [
  community(0, 1435, ["reasoning", "models", "llms"]),
  community(1, 942, ["translation", "machine translation", "machine"]),
  community(2, 840, ["networks", "network", "random"]),
  community(3, 640, ["speech", "recognition", "speech recognition"]),
  // Below the paper's threshold — must be excluded.
  community(4, 120, ["patent", "workshop", "patents"]),
];

const links: CommunityLink[] = [
  { source: 0, target: 1, count: 90 },
  { source: 0, target: 2, count: 40 },
  { source: 1, target: 3, count: 12 },
  { source: 2, target: 3, count: 5 },
  { source: 0, target: 4, count: 80 },
];

test("keeps only communities at or above the size threshold", () => {
  const { nodes } = buildFieldMap(corpus, links);
  expect(nodes).toHaveLength(4);
  expect(nodes.every((n) => n.size >= MIN_COMMUNITY_SIZE)).toBe(true);
  expect(nodes.find((n) => n.id === 4)).toBeUndefined();
});

test("drops links touching an excluded community", () => {
  const { edges } = buildFieldMap(corpus, links);
  expect(edges.every((e) => e.source.id !== 4 && e.target.id !== 4)).toBe(true);
});

test("community discs never overlap after relaxation", () => {
  const { nodes } = buildFieldMap(corpus, links);
  for (let i = 0; i < nodes.length; i++) {
    for (let j = i + 1; j < nodes.length; j++) {
      const d = Math.hypot(nodes[i].x - nodes[j].x, nodes[i].y - nodes[j].y);
      expect(d).toBeGreaterThan(nodes[i].r + nodes[j].r);
    }
  }
});

test("radius follows community size, and only the largest are labelled", () => {
  const { nodes } = buildFieldMap(corpus, links);
  const big = nodes.find((n) => n.id === 0)!;
  const small = nodes.find((n) => n.id === 3)!;
  expect(big.r).toBeGreaterThan(small.r);
  expect(big.labelled).toBe(true);
});

test("layout is deterministic, so the figure reproduces", () => {
  const a = buildFieldMap(corpus, links).nodes.map((n) => [n.x, n.y]);
  const b = buildFieldMap(corpus, links).nodes.map((n) => [n.x, n.y]);
  expect(a).toEqual(b);
  expect(a.every(([x, y]) => Number.isFinite(x) && Number.isFinite(y))).toBe(true);
});

test("an empty or all-small corpus yields an empty map rather than throwing", () => {
  expect(buildFieldMap([], []).nodes).toHaveLength(0);
  expect(buildFieldMap([community(9, 5, ["tiny"])], []).nodes).toHaveLength(0);
});

test("label pairs the head term with the first non-duplicate", () => {
  expect(displayLabel(community(0, 1, ["recommendation", "recommender", "user"]))).toBe(
    "recommendation / user",
  );
  expect(displayLabel(community(0, 1, ["speech", "recognition"]))).toBe("speech / recognition");
});
