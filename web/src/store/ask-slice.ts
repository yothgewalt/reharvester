import type { StateCreator } from "zustand";

import { transport } from "@/lib/api/transport";
import type { AskResponse } from "@/types/domain";

import type { AppState } from "./index";

let askSeq = 0;
let inFlight: AbortController | null = null;

export interface AskSlice {
  askQuestion: string;
  askResult: AskResponse | null;
  askLoading: boolean;
  /** True while prose is still arriving; sources are already on screen. */
  askStreaming: boolean;
  askError: string | null;
  setAskQuestion(q: string): void;
  /** Never rejects; a stale answer is discarded rather than overwriting a newer one. */
  submitAsk(): Promise<void>;
}

export const createAskSlice: StateCreator<AppState, [], [], AskSlice> = (set, get) => ({
  askQuestion: "",
  askResult: null,
  askLoading: false,
  askStreaming: false,
  askError: null,

  setAskQuestion: (askQuestion) => set({ askQuestion }),

  submitAsk: async () => {
    const question = get().askQuestion.trim();
    if (!question) return;

    // Retrieval takes milliseconds and generation takes seconds, so a second
    // question would otherwise sit behind the first model call. Abandon it.
    inFlight?.abort();
    const controller = new AbortController();
    inFlight = controller;

    const seq = ++askSeq;
    set({ askLoading: true, askStreaming: false, askError: null, askResult: null });

    try {
      await transport.askStream(
        { question },
        {
          onSources: (meta) => {
            if (seq !== askSeq) return;
            // Sources are the useful half and they are ready now; show them
            // rather than holding the page blank until the prose exists.
            set({
              askResult: { ...meta, answer: "" },
              askLoading: false,
              askStreaming: true,
            });
          },
          onToken: (text) => {
            if (seq !== askSeq) return;
            const current = get().askResult;
            if (!current) return;
            set({ askResult: { ...current, answer: current.answer + text } });
          },
          onDone: (answer) => {
            if (seq !== askSeq) return;
            const current = get().askResult;
            // Prefer the server's final string: it is authoritative, and a
            // dropped frame would otherwise leave a hole in the middle.
            set({
              askResult: current ? { ...current, answer: answer || current.answer } : current,
              askLoading: false,
              askStreaming: false,
            });
          },
          onError: (message) => {
            if (seq !== askSeq) return;
            set({ askError: message, askLoading: false, askStreaming: false });
          },
        },
        controller.signal,
      );
    } catch (err) {
      if (seq !== askSeq) return;
      set({
        askLoading: false,
        askStreaming: false,
        askError: err instanceof Error ? err.message : "Ask failed",
      });
    } finally {
      if (inFlight === controller) inFlight = null;
    }
  },
});
