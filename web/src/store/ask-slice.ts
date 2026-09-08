import type { StateCreator } from "zustand";

import { transport } from "@/lib/api/transport";
import type { AskResponse } from "@/types/domain";

import type { AppState } from "./index";

let askSeq = 0;

export interface AskSlice {
  askQuestion: string;
  askResult: AskResponse | null;
  askLoading: boolean;
  askError: string | null;
  setAskQuestion(q: string): void;
  /** Never rejects; a stale answer is discarded rather than overwriting a newer one. */
  submitAsk(): Promise<void>;
}

export const createAskSlice: StateCreator<AppState, [], [], AskSlice> = (set, get) => ({
  askQuestion: "",
  askResult: null,
  askLoading: false,
  askError: null,

  setAskQuestion: (askQuestion) => set({ askQuestion }),

  submitAsk: async () => {
    const question = get().askQuestion.trim();
    if (!question) return;
    const seq = ++askSeq;
    set({ askLoading: true, askError: null });
    try {
      const askResult = await transport.ask({ question });
      if (seq !== askSeq) return;
      set({ askResult, askLoading: false });
    } catch (err) {
      if (seq !== askSeq) return;
      set({
        askLoading: false,
        askError: err instanceof Error ? err.message : "Ask failed",
      });
    }
  },
});
