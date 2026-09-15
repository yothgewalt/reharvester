"use client";
import AddOutlined from "@mui/icons-material/AddOutlined";
import CenterFocusStrongOutlined from "@mui/icons-material/CenterFocusStrongOutlined";
import RemoveOutlined from "@mui/icons-material/RemoveOutlined";
import IconButton from "@mui/material/IconButton";
import Tooltip from "@mui/material/Tooltip";
import type { MultiDirectedGraph } from "graphology";
import type FA2LayoutSupervisor from "graphology-layout-forceatlas2/worker";
import type Sigma from "sigma";
import type { Settings } from "sigma/settings";
import type { CameraState, NodeDisplayData, PartialButFor } from "sigma/types";
import { useEffect, useEffectEvent, useRef, useState } from "react";

import { usePrefersReducedMotion } from "@/components/layout/usePrefersReducedMotion";
import { color } from "@/theme/tokens";
import type { GraphEdge, GraphNode, GraphRelation } from "@/types/domain";

import {
  type Adjacency,
  type Direction,
  focusRoles,
  type FocusRoles,
  MUTUAL,
  type Role,
  seedPositions,
} from "./graph-model";

export interface Hover {
  node: string;
  edge: string;
}

interface NodeAttrs {
  x: number;
  y: number;
  size: number;
  color: string;
  label: string;
  kind: "paper" | "concept";
}

interface EdgeAttrs {
  relation: GraphRelation;
  weight: number;
  size: number;
  color: string;
}

type Graph = MultiDirectedGraph<NodeAttrs, EdgeAttrs>;
type Renderer = Sigma<NodeAttrs, EdgeAttrs>;
type LabelData = PartialButFor<NodeDisplayData, "x" | "y" | "size" | "label" | "color">;

interface View {
  focus: string | null;
  roles: FocusRoles;
  labelled: Set<string>;
  hover: Hover | null;
  relations: ReadonlySet<GraphRelation>;
}

interface Libs {
  FA2: typeof FA2LayoutSupervisor;
  inferSettings: typeof import("graphology-layout-forceatlas2").inferSettings;
}

/** Under 5 s, so A11Y.md needs no pause control for the settling motion. */
const LAYOUT_MS = 3000;
const LABEL_CAP = 12;
const TITLE_CAP = 48;
const CAMERA_MS = 250;
const ROLE_COLOR: Record<Role, string> = { out: color.ink, in: color.accent, mutual: color.ink3 };
// ink3 at 75% alpha: visible on its own, and overlaps still darken so dense structure reads.
const CONTEXT_EDGE = `${color.ink3}BF`;
const NO_ROLES: FocusRoles = { nodes: new Map(), edges: new Map() };

// The graph (with its settled layout) outlives the component, so returning to
// the project's Graph tab renders instantly instead of re-laying out.
const cache: { graph: Graph | null; settled: boolean } = { graph: null, settled: false };

function cachedGraph(create: () => Graph): Graph {
  cache.graph ??= create();
  return cache.graph;
}

function markSettled(settled: boolean) {
  cache.settled = settled;
}

/** Adds what the store gained since the last sync; clears first if the cached
 *  graph holds nodes or edges the store no longer has (a different project). */
function syncGraph(
  graph: Graph,
  nodes: readonly GraphNode[],
  edges: readonly GraphEdge[],
  adjacency: ReadonlyMap<string, Adjacency>,
): { changed: boolean; addedNodes: number } {
  const ids = new Set(nodes.map((n) => n.id));
  const edgeIds = new Set(edges.map((e) => e.id));
  let changed = false;
  if (graph.someNode((id) => !ids.has(id)) || graph.someEdge((id) => !edgeIds.has(id))) {
    graph.clear();
    markSettled(false);
    changed = true;
  }

  const missing = nodes.filter((n) => !graph.hasNode(n.id));
  if (missing.length > 0) {
    const fresh = graph.order === 0;
    const seeds = seedPositions(nodes);
    const maxRank = nodes.reduce((m, n) => Math.max(m, n.data.pageRank), 0) || 1;
    for (const n of missing) {
      const concept = n.data.kind === "concept";
      graph.addNode(n.id, {
        ...(fresh ? seeds.get(n.id)! : nearPlaced(graph, adjacency, n.id, seeds)),
        label: n.label,
        kind: n.data.kind,
        size: concept ? 4 : 1.5 + 5 * Math.sqrt(n.data.pageRank / maxRank),
        color: concept ? color.ink : color.ink3,
      });
    }
    changed = true;
  }

  for (const e of edges) {
    if (e.source === e.target || graph.hasEdge(e.id)) continue;
    if (!graph.hasNode(e.source) || !graph.hasNode(e.target)) continue;
    graph.addDirectedEdgeWithKey(e.id, e.source, e.target, {
      relation: e.relation,
      weight: e.weight,
      size: 0.5,
      color: CONTEXT_EDGE,
    });
    changed = true;
  }
  return { changed, addedNodes: missing.length };
}

// ponytail: first placed neighbour + jitter; a crawl that adds many unlinked
// nodes falls back to seed coordinates, which the next layout run corrects.
function nearPlaced(
  graph: Graph,
  adjacency: ReadonlyMap<string, Adjacency>,
  id: string,
  seeds: Map<string, { x: number; y: number }>,
) {
  const a = adjacency.get(id);
  const anchor = [...(a?.out ?? []), ...(a?.in ?? []), ...(a?.mutual ?? [])].find((l) => graph.hasNode(l.other));
  if (!anchor) return seeds.get(id)!;
  const { x, y } = graph.getNodeAttributes(anchor.other);
  return { x: x + Math.random() - 0.5, y: y + Math.random() - 0.5 };
}

function makeSettings(host: HTMLElement, view: { current: View }): Partial<Settings<NodeAttrs, EdgeAttrs>> {
  const style = getComputedStyle(host);
  const sans = style.fontFamily;
  const mono = `${style.getPropertyValue("--font-plex-mono") || "ui-monospace"}, monospace`;

  const drawLabel = (ctx: CanvasRenderingContext2D, d: LabelData, s: Settings<NodeAttrs, EdgeAttrs>) => {
    if (!d.label) return;
    const concept = d.kind === "concept";
    const text = concept
      ? d.label.toUpperCase()
      : d.label.length > TITLE_CAP
        ? `${d.label.slice(0, TITLE_CAP - 1)}…`
        : d.label;
    ctx.fillStyle = color.ink;
    ctx.font = `${s.labelWeight} ${s.labelSize}px ${concept ? mono : sans}`;
    ctx.fillText(text, d.x + d.size + 6, d.y + s.labelSize / 3);
  };

  // Sigma's default hover box has a blurred shadow; the theme separates with
  // 1px rings instead.
  const drawHover = (ctx: CanvasRenderingContext2D, d: LabelData, s: Settings<NodeAttrs, EdgeAttrs>) => {
    const v = view.current;
    if (d.key === v.focus || d.key === v.hover?.node) {
      ctx.beginPath();
      ctx.arc(d.x, d.y, d.size + 3, 0, Math.PI * 2);
      ctx.strokeStyle = color.slab;
      ctx.lineWidth = 2;
      ctx.stroke();
    }
    if (d.label) {
      ctx.font = `${s.labelWeight} ${s.labelSize}px ${d.kind === "concept" ? mono : sans}`;
      const shown = d.kind === "concept" ? d.label.toUpperCase() : d.label.slice(0, TITLE_CAP);
      const width = ctx.measureText(shown).width + 12;
      const height = s.labelSize + 10;
      ctx.beginPath();
      ctx.roundRect(d.x + d.size + 1, d.y - height / 2, width, height, 6);
      ctx.fillStyle = color.white;
      ctx.fill();
      ctx.strokeStyle = color.line;
      ctx.lineWidth = 1;
      ctx.stroke();
      drawLabel(ctx, d, s);
    }
  };

  return {
    labelFont: sans,
    labelSize: 12,
    labelWeight: "500",
    labelColor: { color: color.ink },
    labelRenderedSizeThreshold: 6,
    labelDensity: 0.4,
    labelGridCellSize: 140,
    defaultNodeColor: color.ink3,
    defaultEdgeColor: CONTEXT_EDGE,
    hideEdgesOnMove: !view.current.focus,
    enableCameraRotation: false,
    minCameraRatio: 0.05,
    maxCameraRatio: 3,
    stagePadding: 24,
    zIndex: false,
    defaultDrawNodeLabel: drawLabel,
    defaultDrawNodeHover: drawHover,
    nodeReducer: (id, a) => {
      const v = view.current;
      if (!v.focus) return a;
      // Never change size or position here: focus-to-focus refreshes skip
      // re-indexing, which is only sound while those stay fixed.
      if (id === v.focus) return { ...a, color: color.slab, highlighted: true, forceLabel: true };
      const role = v.roles.nodes.get(id);
      if (!role) return { ...a, color: color.line, label: null };
      const hovered = v.hover?.node === id;
      return { ...a, color: ROLE_COLOR[role], highlighted: hovered, forceLabel: hovered || v.labelled.has(id) };
    },
    // Edge program type may only change on a full refresh, so it depends on
    // whether anything is focused, never on which node.
    edgeReducer: (id, a) => {
      const v = view.current;
      if (!v.focus) return v.relations.has(a.relation) ? a : { ...a, hidden: true };
      const type = MUTUAL.has(a.relation) ? "line" : "arrow";
      const role = v.roles.edges.get(id);
      if (!role) return { ...a, type, hidden: true };
      if (v.hover?.edge === id) return { ...a, type, color: color.slab, size: 4 };
      return { ...a, type, color: ROLE_COLOR[role], size: role === "mutual" ? 2 : 3 };
    },
  };
}

function moveCamera(renderer: Renderer, state: Partial<CameraState>, reduced: boolean) {
  const camera = renderer.getCamera();
  // duration 0 yields a NaN frame in sigma; setState is the no-motion path.
  if (reduced) camera.setState(state);
  else void camera.animate(state, { duration: CAMERA_MS });
}

function centreOn(renderer: Renderer, id: string, reduced: boolean) {
  const d = renderer.getNodeDisplayData(id);
  if (!d) return;
  moveCamera(renderer, { x: d.x, y: d.y, ratio: Math.min(renderer.getCamera().ratio, 0.4) }, reduced);
}

export interface GraphCanvasProps {
  nodes: readonly GraphNode[];
  edges: readonly GraphEdge[];
  adjacency: ReadonlyMap<string, Adjacency>;
  focusId: string | null;
  hover: Hover | null;
  mode: Direction;
  relations: ReadonlySet<GraphRelation>;
  /** id of the visible text that names the drawing (aria-labelledby) */
  captionId: string;
  onFocusChange(id: string | null): void;
}

/**
 * WebGL drawing of the graph. Everything it shows is also reachable through
 * the DOM (search + neighbour lists), so it is presented as one labelled image.
 */
export function GraphCanvas({
  nodes,
  edges,
  adjacency,
  focusId,
  hover,
  mode,
  relations,
  captionId,
  onFocusChange,
}: GraphCanvasProps) {
  const reduced = usePrefersReducedMotion();
  const frameRef = useRef<HTMLDivElement | null>(null);
  const hostRef = useRef<HTMLDivElement | null>(null);
  const rendererRef = useRef<Renderer | null>(null);
  const libsRef = useRef<Libs | null>(null);
  const layoutRef = useRef<{ fa2: FA2LayoutSupervisor; timer: number } | null>(null);
  const clickedRef = useRef<string | null>(null);
  const view = useRef<View>({ focus: null, roles: NO_ROLES, labelled: new Set(), hover: null, relations });
  const [status, setStatus] = useState<"loading" | "ready" | "unavailable">("loading");

  const focusFromCanvas = useEffectEvent((id: string | null) => {
    clickedRef.current = id;
    onFocusChange(id);
  });

  const stopLayout = useEffectEvent((settled: boolean) => {
    const layout = layoutRef.current;
    if (!layout) return;
    window.clearTimeout(layout.timer);
    layout.fa2.kill();
    layoutRef.current = null;
    if (frameRef.current) frameRef.current.dataset.settling = "false";
    if (!settled) return;
    markSettled(true);
    const renderer = rendererRef.current;
    if (renderer && view.current.focus) centreOn(renderer, view.current.focus, reduced);
  });

  const startLayout = useEffectEvent(() => {
    const renderer = rendererRef.current;
    const libs = libsRef.current;
    if (!renderer || !libs || layoutRef.current) return;
    const graph = renderer.getGraph() as Graph;
    if (graph.order === 0) return;
    try {
      const fa2 = new libs.FA2(graph, { settings: libs.inferSettings(graph) });
      fa2.start();
      if (frameRef.current) frameRef.current.dataset.settling = "true";
      layoutRef.current = { fa2, timer: window.setTimeout(() => stopLayout(true), LAYOUT_MS) };
    } catch {
      markSettled(true); // the seeded community layout stays readable
    }
  });

  useEffect(() => {
    const host = hostRef.current;
    if (!host) return;
    let dead = false;
    let renderer: Renderer | null = null;
    void (async () => {
      try {
        const [{ default: SigmaCtor }, { MultiDirectedGraph: GraphCtor }, { default: FA2 }, { inferSettings }] =
          await Promise.all([
            import("sigma"),
            import("graphology"),
            import("graphology-layout-forceatlas2/worker"),
            import("graphology-layout-forceatlas2"),
          ]);
        if (dead) return;
        const graph = cachedGraph(() => new GraphCtor<NodeAttrs, EdgeAttrs>());
        renderer = new SigmaCtor<NodeAttrs, EdgeAttrs>(graph, host, makeSettings(host, view));
        renderer.on("clickNode", ({ node }) => focusFromCanvas(node));
        renderer.on("clickStage", () => focusFromCanvas(null));
        renderer.on("enterNode", () => (host.style.cursor = "pointer"));
        renderer.on("leaveNode", () => (host.style.cursor = ""));
        rendererRef.current = renderer;
        libsRef.current = { FA2, inferSettings };
        setStatus("ready");
      } catch {
        if (dead) return;
        renderer?.kill();
        host.replaceChildren();
        setStatus("unavailable");
      }
    })();
    return () => {
      dead = true;
      stopLayout(false);
      renderer?.kill();
      rendererRef.current = null;
    };
  }, []);

  useEffect(() => {
    const renderer = rendererRef.current;
    if (status !== "ready" || !renderer) return;
    const { changed, addedNodes } = syncGraph(renderer.getGraph() as Graph, nodes, edges, adjacency);
    if (changed) renderer.refresh();
    if (addedNodes > 0 || !cache.settled) startLayout();
  }, [status, nodes, edges, adjacency]);

  useEffect(() => {
    const renderer = rendererRef.current;
    if (status !== "ready" || !renderer) return;
    const graph = renderer.getGraph();
    const focus = focusId && graph.hasNode(focusId) ? focusId : null;
    const roles = focus ? focusRoles(adjacency, focus, mode, relations) : NO_ROLES;
    const prev = view.current;
    const next: View = {
      focus,
      roles,
      labelled: new Set([...roles.nodes.keys()].slice(0, LABEL_CAP)),
      hover: focus ? hover : null,
      relations,
    };
    view.current = next;
    if (!prev.focus !== !next.focus) renderer.setSetting("hideEdgesOnMove", !next.focus);

    if (prev.focus && next.focus) {
      // Focus-to-focus touches only the two neighbourhoods: O(degree), not O(E).
      const nodeIds = new Set([...prev.roles.nodes.keys(), ...next.roles.nodes.keys(), prev.focus, next.focus]);
      const edgeIds = new Set([...prev.roles.edges.keys(), ...next.roles.edges.keys()]);
      for (const h of [prev.hover, next.hover]) {
        if (!h) continue;
        nodeIds.add(h.node);
        edgeIds.add(h.edge);
      }
      renderer.refresh({
        partialGraph: {
          nodes: [...nodeIds].filter((id) => graph.hasNode(id)),
          edges: [...edgeIds].filter((id) => graph.hasEdge(id)),
        },
        skipIndexation: true,
      });
    } else if (prev.focus !== next.focus || prev.relations !== next.relations) {
      renderer.refresh();
    }

    if (next.focus && next.focus !== prev.focus && clickedRef.current !== next.focus) {
      centreOn(renderer, next.focus, reduced);
    }
    clickedRef.current = null;
  }, [status, adjacency, focusId, hover, mode, relations, reduced]);

  useEffect(() => {
    const renderer = rendererRef.current;
    if (status !== "ready" || !renderer) return;
    renderer.setSettings(
      reduced
        ? { inertiaRatio: 0, zoomDuration: 1, doubleClickZoomingDuration: 1 }
        : { inertiaRatio: 3, zoomDuration: CAMERA_MS, doubleClickZoomingDuration: 200 },
    );
  }, [status, reduced]);

  const zoom = (factor: number | null) => {
    const renderer = rendererRef.current;
    if (!renderer) return;
    const camera = renderer.getCamera();
    moveCamera(
      renderer,
      factor === null
        ? { x: 0.5, y: 0.5, ratio: 1, angle: 0 }
        : { ratio: camera.getBoundedRatio(camera.ratio * factor) },
      reduced,
    );
  };

  return (
    <div ref={frameRef} data-settling="false" className="group relative h-full w-full">
      <div
        ref={hostRef}
        role={status === "ready" ? "img" : undefined}
        aria-labelledby={status === "ready" ? captionId : undefined}
        className="absolute inset-0 group-data-[settling=true]:invisible motion-safe:group-data-[settling=true]:visible"
      />
      <p className="pointer-events-none absolute inset-0 m-0 hidden items-center justify-center font-mono text-[13px] text-ink-3 group-data-[settling=true]:flex motion-safe:group-data-[settling=true]:hidden">
        Arranging {nodes.length.toLocaleString()} nodes…
      </p>
      {status === "unavailable" ? (
        <p className="absolute inset-0 m-0 flex items-center justify-center p-8 text-center text-sm text-ink-2">
          This browser can&apos;t draw the graph because WebGL is unavailable. Search and the
          relationship lists still work.
        </p>
      ) : null}
      {status === "ready" ? (
        <div className="absolute right-3 top-3 flex flex-col rounded-md bg-white ring-line">
          <Tooltip title="Zoom in" placement="left">
            <IconButton size="small" onClick={() => zoom(1 / 1.5)}>
              <AddOutlined fontSize="small" />
            </IconButton>
          </Tooltip>
          <Tooltip title="Zoom out" placement="left">
            <IconButton size="small" onClick={() => zoom(1.5)}>
              <RemoveOutlined fontSize="small" />
            </IconButton>
          </Tooltip>
          <Tooltip title="Fit graph" placement="left">
            <IconButton size="small" onClick={() => zoom(null)}>
              <CenterFocusStrongOutlined fontSize="small" />
            </IconButton>
          </Tooltip>
        </div>
      ) : null}
    </div>
  );
}
