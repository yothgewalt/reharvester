import type { StateCreator } from "zustand";

import { transport } from "@/lib/api/transport";
import type { GapReport, TrendKeyword } from "@/types/domain";

import type { AppState } from "./index";

export const TREND_CURRENT_YEAR = 2026;

let trendsSeq = 0;

export interface TrendsSlice {
  selectedTimeWindow: [number, number];
  emergingKeywords: TrendKeyword[];
  trendsLoading: boolean;
  gapReport: GapReport | null;
  setTimeWindow(w: [number, number]): void;
  recalculateTrends(): Promise<void>;
  loadGapPairs(): Promise<void>;
}

export const createTrendsSlice: StateCreator<AppState, [], [], TrendsSlice> = (set, get) => ({
  selectedTimeWindow: [TREND_CURRENT_YEAR - 2, TREND_CURRENT_YEAR],
  emergingKeywords: [],
  trendsLoading: false,
  gapReport: null,

  setTimeWindow: (selectedTimeWindow) => set({ selectedTimeWindow }),

  loadGapPairs: async () => {
    if (get().gapReport) return;
    try {
      set({ gapReport: await transport.listGapPairs() });
    } catch {
      // The page still works without it; the header reports backend trouble.
    }
  },

  recalculateTrends: async () => {
    const seq = ++trendsSeq;
    set({ trendsLoading: true });
    try {
      const emergingKeywords = await transport.recalculateTrends(get().selectedTimeWindow);
      if (seq !== trendsSeq) return;
      set({ emergingKeywords, trendsLoading: false });
    } catch {
      if (seq !== trendsSeq) return;
      set({ trendsLoading: false });
    }
  },
});
