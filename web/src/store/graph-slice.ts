import type { StateCreator } from "zustand";

import { transport } from "@/lib/api/transport";
import type { Community, CommunityLink, GraphEdge, GraphNode } from "@/types/domain";

import type { AppState } from "./index";





let graphCache: { nodes: GraphNode[]; edges: GraphEdge[] } | null = null;

export interface GraphSlice {
  nodes: GraphNode[];
  edges: GraphEdge[];
  graphLoading: boolean;
  graphError: string | null;
  selectedNodeId: string | null;
  selectedDocId: string | null;
  communities: Community[];
  communityLinks: CommunityLink[];
  communitiesLoading: boolean;
  selectedCommunitySlug: string | null;
  loadCommunities(force?: boolean): Promise<void>;
  resetCorpusView(): void;
  selectCommunity(slug: string | null): void;
  loadGraphSnapshot(): Promise<void>;
  applyGraphDelta(d: { addedNodes: GraphNode[]; addedEdges: GraphEdge[] }): void;
  selectNode(nodeId: string, docId: string): void;
}

export const createGraphSlice: StateCreator<AppState, [], [], GraphSlice> = (set, get) => ({
  nodes: [],
  edges: [],
  graphLoading: false,
  graphError: null,
  selectedNodeId: null,
  selectedDocId: null,

  loadGraphSnapshot: async () => {
    set({ graphLoading: true, graphError: null });
    try {
      const snap = await transport.getGraphSnapshot();

      const cached = graphCache;
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
      graphCache = { nodes, edges };
      set({ nodes, edges, graphLoading: false });
    } catch (err) {
      set({
        graphLoading: false,
        graphError: err instanceof Error ? err.message : "Failed to load graph snapshot",
      });
    }
  },

  applyGraphDelta: ({ addedNodes, addedEdges }) => {

    const cache = graphCache ?? { nodes: [], edges: [] };
    const known = new Set(cache.nodes.map((n) => n.id));
    const knownEdges = new Set(cache.edges.map((e) => e.id));
    const freshNodes = addedNodes.filter((n) => !known.has(n.id));
    freshNodes.forEach((n) => known.add(n.id));
    const freshEdges = addedEdges.filter(
      (e) => !knownEdges.has(e.id) && known.has(e.source) && known.has(e.target),
    );
    if (freshNodes.length === 0 && freshEdges.length === 0) return;
    graphCache = {
      nodes: [...cache.nodes, ...freshNodes],
      edges: [...cache.edges, ...freshEdges],
    };
    set({ nodes: graphCache.nodes, edges: graphCache.edges });
  },

  communities: [],
  communityLinks: [],
  communitiesLoading: false,
  selectedCommunitySlug: null,

  // resetCorpusView drops everything derived from the previous corpus: the
  // community rail, and the node cache that loadGraphSnapshot merges into
  // rather than replaces. Call it when a harvest finishes, otherwise the UI
  // keeps showing the old corpus's communities until a full page reload.
  resetCorpusView: () => {
    graphCache = null;
    set({
      nodes: [],
      edges: [],
      communities: [],
      communityLinks: [],
      selectedCommunitySlug: null,
      selectedNodeId: null,
      selectedDocId: null,
    });
  },

  // Pass force after a rebuild: the cached rail belongs to the corpus that was
  // active when it was fetched, and nothing else invalidates it.
  loadCommunities: async (force = false) => {
    if (get().communitiesLoading) return;
    if (!force && get().communities.length > 0) return;
    set({ communitiesLoading: true });
    try {
      const [communities, communityLinks] = await Promise.all([
        transport.listCommunities(),
        transport.listCommunityLinks(),
      ]);
      set({ communities, communityLinks, communitiesLoading: false });
    } catch {
      // The header already surfaces backend trouble; an empty rail is enough here.
      set({ communitiesLoading: false });
    }
  },

  selectCommunity: (slug) => {
    set({ selectedCommunitySlug: slug });
    if (!slug) return;
    const c = get().communities.find((x) => x.slug === slug);
    const first = c?.memberDocIds[0];
    if (first) get().selectNode(first, first);
  },

  selectNode: (nodeId, docId) => {
    set({ selectedNodeId: nodeId, selectedDocId: docId });
    void get().loadWikiDoc(docId);
  },



});
