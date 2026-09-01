import type { Community, CommunityLink } from "@/types/domain";

/**
 * Layout for the coarse-grained field map (paper Fig. 2c): one node per
 * community, edges weighted by inter-community backbone links.
 *
 * A short deterministic force relaxation — at ~21 nodes this is a few thousand
 * operations, so it runs inline with no dependency and no animation. Seeded
 * from a circle rather than randomly, so the same corpus always maps the same
 * way and the picture is reproducible, which is the point of the figure.
 */

/** The paper's Fig. 2c threshold: communities of at least this many papers. */
export const MIN_COMMUNITY_SIZE = 400;
/** Only the ten largest are labelled, as in the figure. */
export const LABELLED_COUNT = 10;

const ITERATIONS = 400;
const AREA = 1000;

export interface MapNode {
  id: number;
  label: string;
  slug: string;
  size: number;
  x: number;
  y: number;
  r: number;
  labelled: boolean;
}

export interface MapEdge {
  source: MapNode;
  target: MapNode;
  count: number;
}

export interface FieldMap {
  nodes: MapNode[];
  edges: MapEdge[];
  /** Link count below which an edge is dropped, so the map is not a hairball. */
  threshold: number;
}

export function buildFieldMap(
  communities: Community[],
  links: CommunityLink[],
  minSize = MIN_COMMUNITY_SIZE,
): FieldMap {
  const kept = communities
    .filter((c) => c.size >= minSize)
    .sort((a, b) => b.size - a.size);
  if (kept.length === 0) return { nodes: [], edges: [], threshold: 0 };

  const maxSize = kept[0].size;
  const nodes: MapNode[] = kept.map((c, i) => {
    const a = (i / kept.length) * Math.PI * 2;
    return {
      id: c.id,
      label: displayLabel(c),
      slug: c.slug,
      size: c.size,
      // Seeded on a circle; the relaxation below does the rest.
      x: Math.cos(a) * AREA * 0.3,
      y: Math.sin(a) * AREA * 0.3,
      r: 12 + 34 * Math.sqrt(c.size / maxSize),
      labelled: i < LABELLED_COUNT,
    };
  });
  const byId = new Map(nodes.map((n) => [n.id, n]));

  const present = links.filter((l) => byId.has(l.source) && byId.has(l.target));
  // Keep the upper half of link counts: dense enough to show structure,
  // sparse enough to read. The paper thresholds here too but does not give a value.
  const counts = present.map((l) => l.count).sort((a, b) => a - b);
  const threshold = counts.length > 0 ? counts[Math.floor(counts.length * 0.5)] : 0;

  const edges: MapEdge[] = present
    .filter((l) => l.count >= threshold)
    .map((l) => ({ source: byId.get(l.source)!, target: byId.get(l.target)!, count: l.count }));

  relax(nodes, edges);
  return { nodes, edges, threshold };
}

/** Repulsion between all pairs, attraction along edges, cooling step size. */
function relax(nodes: MapNode[], edges: MapEdge[]): void {
  const maxCount = Math.max(1, ...edges.map((e) => e.count));
  for (let step = 0; step < ITERATIONS; step++) {
    const cool = 1 - step / ITERATIONS;
    const fx = new Float64Array(nodes.length);
    const fy = new Float64Array(nodes.length);

    for (let i = 0; i < nodes.length; i++) {
      for (let j = i + 1; j < nodes.length; j++) {
        let dx = nodes[i].x - nodes[j].x;
        let dy = nodes[i].y - nodes[j].y;
        let d2 = dx * dx + dy * dy;
        if (d2 < 1) {
          // Deterministic nudge so coincident nodes still separate.
          dx = (i - j) || 1;
          dy = 1;
          d2 = 2;
        }
        const min = nodes[i].r + nodes[j].r + 24;
        const push = (min * min * 60) / d2;
        const d = Math.sqrt(d2);
        fx[i] += (dx / d) * push;
        fy[i] += (dy / d) * push;
        fx[j] -= (dx / d) * push;
        fy[j] -= (dy / d) * push;
      }
    }

    for (const e of edges) {
      const i = nodes.indexOf(e.source);
      const j = nodes.indexOf(e.target);
      const dx = e.target.x - e.source.x;
      const dy = e.target.y - e.source.y;
      const d = Math.hypot(dx, dy) || 1;
      const pull = (d * (e.count / maxCount)) / 6;
      fx[i] += (dx / d) * pull;
      fy[i] += (dy / d) * pull;
      fx[j] -= (dx / d) * pull;
      fy[j] -= (dy / d) * pull;
    }

    for (let i = 0; i < nodes.length; i++) {
      const step2 = Math.min(40, Math.hypot(fx[i], fy[i])) * cool * 0.35;
      const m = Math.hypot(fx[i], fy[i]) || 1;
      nodes[i].x += (fx[i] / m) * step2;
      nodes[i].y += (fy[i] / m) * step2;
    }
  }
}

/**
 * "reasoning / models" — the first term plus the first that is not a near
 * duplicate, since Louvain labels stack morphological variants.
 */
export function displayLabel(c: Community): string {
  const [head, ...rest] = c.terms;
  if (!head) return c.label;
  const second = rest.find((t) => !related(head, t));
  return second ? `${head} / ${second}` : head;
}

function related(a: string, b: string): boolean {
  const x = a.toLowerCase();
  const y = b.toLowerCase();
  return x.includes(y) || y.includes(x) || x.slice(0, 6) === y.slice(0, 6);
}
