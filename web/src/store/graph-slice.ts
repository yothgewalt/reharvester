import type { StateCreator } from "zustand";

import { transport } from "@/lib/api/transport";
import type { GraphEdge, GraphKind, GraphNode, LayoutPreset } from "@/types/domain";

import type { AppState } from "./index";

export type FocusOrigin = "canvas" | "external" | null;

let gapSeq = 0;



const graphCache: Partial<Record<GraphKind, { nodes: GraphNode[]; edges: GraphEdge[] }>> = {};

export interface GraphSlice {
  nodes: GraphNode[];
  edges: GraphEdge[];
  graphKind: GraphKind;
  graphLoading: boolean;
  graphError: string | null;
  activeLayout: LayoutPreset;
  setGraphKind(kind: GraphKind): void;
  selectedNodeId: string | null;
  selectedDocId: string | null;
  focusOrigin: FocusOrigin;
  isGapAnalysisOverlayActive: boolean;
  gapNodeIds: string[];
  loadGraphSnapshot(): Promise<void>;
  applyGraphDelta(d: { addedNodes: GraphNode[]; addedEdges: GraphEdge[] }): void;
  setLayout(l: LayoutPreset): void;
  selectNode(nodeId: string, docId: string, origin: Exclude<FocusOrigin, null>): void;
  clearSelection(): void;
  toggleGapOverlay(): Promise<void>;
}

export const createGraphSlice: StateCreator<AppState, [], [], GraphSlice> = (set, get) => ({
  nodes: [],
  edges: [],
  graphKind: "knowledge",
  graphLoading: false,
  graphError: null,
  activeLayout: "forceDirected2d",
  selectedNodeId: null,
  selectedDocId: null,
  focusOrigin: null,
  isGapAnalysisOverlayActive: false,
  gapNodeIds: [],

  setGraphKind: (kind) => {
    if (get().graphKind === kind) return;
    get().clearSelection();
    set({
      graphKind: kind,

      ...(kind !== "knowledge" ? { isGapAnalysisOverlayActive: false } : {}),
    });
    const cached = graphCache[kind];
    if (cached) {
      set({ nodes: cached.nodes, edges: cached.edges, graphError: null });
    } else {
      void get().loadGraphSnapshot();
    }
  },

  loadGraphSnapshot: async () => {
    const kind = get().graphKind;
    set({ graphLoading: true, graphError: null });
    try {
      const snap = await transport.getGraphSnapshot(kind);


      const cached = graphCache[kind];
      let nodes = snap.nodes;
      let edges = snap.edges;
      if (cached && cached.nodes.length > 0) {
        const snapIds = new Set(snap.nodes.map((n) => n.id));
        nodes = [...snap.nodes, ...cached.nodes.filter((n) => !snapIds.has(n.id))];
        const snapEdgeIds = new Set(snap.edges.map((e) => e.id));
        const nodeIds = new Set(nodes.map((n) => n.id));
        edges = [
          ...snap.edges,
          ...cached.edges.filter(
            (e) => !snapEdgeIds.has(e.id) && nodeIds.has(e.source) && nodeIds.has(e.target),
          ),
        ];
      }
      graphCache[kind] = { nodes, edges };
      if (get().graphKind !== kind) return;
      set({ nodes, edges, graphLoading: false });
    } catch (err) {
      if (get().graphKind !== kind) return;
      set({
        graphLoading: false,
        graphError: err instanceof Error ? err.message : "Failed to load graph snapshot",
      });
    }
  },

  applyGraphDelta: ({ addedNodes, addedEdges }) => {

    const cache = graphCache.knowledge ?? { nodes: [], edges: [] };
    const known = new Set(cache.nodes.map((n) => n.id));
    const knownEdges = new Set(cache.edges.map((e) => e.id));
    const freshNodes = addedNodes.filter((n) => !known.has(n.id));
    freshNodes.forEach((n) => known.add(n.id));
    const freshEdges = addedEdges.filter(
      (e) => !knownEdges.has(e.id) && known.has(e.source) && known.has(e.target),
    );
    if (freshNodes.length === 0 && freshEdges.length === 0) return;
    graphCache.knowledge = {
      nodes: [...cache.nodes, ...freshNodes],
      edges: [...cache.edges, ...freshEdges],
    };
    if (get().graphKind === "knowledge") {
      set({ nodes: graphCache.knowledge.nodes, edges: graphCache.knowledge.edges });
    }
  },

  setLayout: (activeLayout) => set({ activeLayout }),

  selectNode: (nodeId, docId, origin) => {
    set({ selectedNodeId: nodeId, selectedDocId: docId, focusOrigin: origin });
    void get().loadWikiDoc(docId);
  },

  clearSelection: () => set({ selectedNodeId: null, focusOrigin: null }),

  toggleGapOverlay: async () => {
    const turningOn = !get().isGapAnalysisOverlayActive;
    const seq = ++gapSeq;
    set({ isGapAnalysisOverlayActive: turningOn });
    if (turningOn && get().gapNodeIds.length === 0) {
      try {
        const res = await transport.getGapPositions();
        if (seq !== gapSeq) return;
        set({ gapNodeIds: res.gapNodeIds });
      } catch {
        if (seq !== gapSeq) return;
        set({ isGapAnalysisOverlayActive: false });
      }
    }
  },
});
