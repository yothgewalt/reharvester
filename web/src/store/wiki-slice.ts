import type { StateCreator } from "zustand";

import { transport } from "@/lib/api/transport";
import type { WikiDoc } from "@/types/domain";

import type { AppState } from "./index";

export interface WikiSlice {
  wikiDoc: WikiDoc | null;
  wikiLoading: boolean;
  wikiError: string | null;
  loadWikiDoc(docId: string): Promise<void>;
}

export const createWikiSlice: StateCreator<AppState, [], [], WikiSlice> = (set, get) => ({
  wikiDoc: null,
  wikiLoading: false,
  wikiError: null,

  loadWikiDoc: async (docId) => {
    set({ wikiLoading: true, wikiError: null });
    try {
      const markdown = await transport.getWikiRaw(docId);

      if (get().selectedDocId !== docId) return;
      const heading = markdown.match(/^#\s+(.+)$/m);
      set({
        wikiDoc: { id: docId, title: heading?.[1]?.trim() ?? docId, markdown },
        wikiLoading: false,
      });
    } catch (err) {
      if (get().selectedDocId !== docId) return;
      set({
        wikiLoading: false,
        wikiError: err instanceof Error ? err.message : "Failed to load document",
      });
    }
  },
});
