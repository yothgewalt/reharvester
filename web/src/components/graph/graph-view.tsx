"use client";



import { useEffect, useMemo, useRef } from "react";
import { GraphCanvas } from "reagraph";
import type {
  GraphCanvasRef,
  GraphNode as ReagraphNode,
  InternalGraphNode,
} from "reagraph";

import { GRAPH_CONFIG } from "@/lib/graph-config";
import "@/lib/silence-three-deprecation";
import { generateGraph } from "@/mocks/fixtures/graph";
import { useAppStore } from "@/store";

import { buildAdjacencyIndex } from "./adjacency";
import { graphTheme } from "./graph-theme";

export interface GraphViewProps {
  reducedMotion: boolean;

  onContextLost?: () => void;
}

export default function GraphView({ reducedMotion, onContextLost }: GraphViewProps) {
  const storeNodes = useAppStore((s) => s.nodes);
  const storeEdges = useAppStore((s) => s.edges);
  const graphKind = useAppStore((s) => s.graphKind);
  const activeLayout = useAppStore((s) => s.activeLayout);
  const selectedNodeId = useAppStore((s) => s.selectedNodeId);
  const focusOrigin = useAppStore((s) => s.focusOrigin);
  const isGapAnalysisOverlayActive = useAppStore((s) => s.isGapAnalysisOverlayActive);
  const gapNodeIds = useAppStore((s) => s.gapNodeIds);
  const selectNode = useAppStore((s) => s.selectNode);
  const clearSelection = useAppStore((s) => s.clearSelection);

  const graphRef = useRef<GraphCanvasRef | null>(null);
  const wrapperRef = useRef<HTMLDivElement | null>(null);
  const onContextLostRef = useRef(onContextLost);
  onContextLostRef.current = onContextLost;


  const stressCount = useMemo(() => {
    if (typeof window === "undefined") return 0;
    const raw = new URLSearchParams(window.location.search).get("nodes");
    const parsed = raw ? Number.parseInt(raw, 10) : 0;
    return Number.isFinite(parsed) && parsed > 0 ? parsed : 0;
  }, []);
  const stressGraph = useMemo(
    () => (stressCount > storeNodes.length ? generateGraph(stressCount) : null),
    [stressCount, storeNodes.length],
  );
  const nodes = stressGraph ? stressGraph.nodes : storeNodes;
  const edges = stressGraph ? stressGraph.edges : storeEdges;

  const adjacency = useMemo(() => buildAdjacencyIndex(edges), [edges]);

  const gapSet = useMemo(() => new Set(gapNodeIds), [gapNodeIds]);
  const gapsOn = isGapAnalysisOverlayActive;
  const renderNodes = useMemo<ReagraphNode[]>(
    () =>
      nodes.map((n) => ({
        id: n.id,
        label: n.label,
        ...(gapsOn && gapSet.has(n.id) ? { fill: GRAPH_CONFIG.gapColor } : {}),
        data: n.data,
      })),
    [nodes, gapsOn, gapSet],
  );




  const renderEdges = useMemo(() => {
    if (graphKind === "cooccurrence") {
      const maxWeight = edges.reduce((m, e) => Math.max(m, e.weight), 1);
      return edges.map((e) => ({ ...e, size: 1 + (e.weight / maxWeight) * 3 }));
    }
    if (!selectedNodeId) return edges;
    const incident = new Set(adjacency.get(selectedNodeId)?.edges ?? []);
    return edges.map((e) => (incident.has(e.id) ? { ...e, label: e.relation } : e));
  }, [edges, graphKind, selectedNodeId, adjacency]);

  const selections = useMemo(
    () => (selectedNodeId ? [selectedNodeId] : []),
    [selectedNodeId],
  );
  const actives = useMemo(() => {
    if (!selectedNodeId) return [];
    const adj = adjacency.get(selectedNodeId);
    return adj ? [...adj.nodes, ...adj.edges] : [];
  }, [selectedNodeId, adjacency]);


  const nodeCount = nodes.length;
  const labelType =
    nodeCount > GRAPH_CONFIG.labelMaxNodes
      ? "none"
      : graphKind === "knowledge" && selectedNodeId
        ? "all"
        : "auto";
  const animated = !reducedMotion && nodeCount <= GRAPH_CONFIG.perfMaxNodes;
  const edgeArrowPosition =
    graphKind === "cooccurrence" || nodeCount > GRAPH_CONFIG.perfMaxNodes ? "none" : "end";


  const adjacencyRef = useRef(adjacency);
  adjacencyRef.current = adjacency;
  const reducedMotionRef = useRef(reducedMotion);
  reducedMotionRef.current = reducedMotion;


  useEffect(() => {
    if (!selectedNodeId || !focusOrigin) return;
    const ref = graphRef.current;
    if (!ref) return;
    if (focusOrigin === "external") {
      ref.centerGraph([selectedNodeId], { animated: !reducedMotionRef.current });
    } else {

      const adj = adjacencyRef.current.get(selectedNodeId);
      ref.fitNodesInView([selectedNodeId, ...(adj?.nodes ?? [])], {
        animated: !reducedMotionRef.current,
      });
    }
  }, [selectedNodeId, focusOrigin]);


  useEffect(() => {
    const id = window.setTimeout(() => {
      graphRef.current?.fitNodesInView(undefined, {
        animated: !reducedMotionRef.current,
      });
    }, 0);
    return () => window.clearTimeout(id);
  }, [activeLayout]);


  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") clearSelection();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [clearSelection]);


  useEffect(() => {
    const wrapper = wrapperRef.current;
    if (!wrapper) return;
    let canvas: HTMLCanvasElement | null = null;
    let raf = 0;
    let tries = 0;
    const handler = () => onContextLostRef.current?.();
    const attach = () => {
      canvas = wrapper.querySelector("canvas");
      if (canvas) {
        canvas.addEventListener("webglcontextlost", handler);
      } else if (tries++ < 30) {
        raf = requestAnimationFrame(attach);
      }
    };
    attach();
    return () => {
      cancelAnimationFrame(raf);
      canvas?.removeEventListener("webglcontextlost", handler);
    };
  }, []);

  return (
    <div ref={wrapperRef} className="relative h-[65vh] min-h-[480px]">
      <GraphCanvas
        ref={graphRef}
        nodes={renderNodes}
        edges={renderEdges}
        theme={graphTheme}
        layoutType={activeLayout}
        sizingType={graphKind === "cooccurrence" ? "attribute" : "pagerank"}
        sizingAttribute={graphKind === "cooccurrence" ? "frequency" : undefined}
        edgeInterpolation={graphKind === "cooccurrence" ? "curved" : "linear"}
        minNodeSize={GRAPH_CONFIG.minNodeSize}
        maxNodeSize={GRAPH_CONFIG.maxNodeSize}

        clusterAttribute={activeLayout.startsWith("forceDirected") ? "cluster" : undefined}
        labelType={labelType}
        animated={animated}
        edgeArrowPosition={edgeArrowPosition}
        selections={selections}
        actives={actives}
        onNodeClick={(node: InternalGraphNode) => selectNode(node.id, node.id, "canvas")}
        onCanvasClick={() => clearSelection()}
      />
      {stressGraph ? (
        <div className="pointer-events-none absolute left-4 top-4 rounded-md bg-slab/80 px-2 py-1 font-mono text-[11px] uppercase tracking-wider text-ink-4">
          stress fixture · {nodes.length.toLocaleString()} nodes ·{" "}
          {edges.length.toLocaleString()} edges
        </div>
      ) : null}
    </div>
  );
}
