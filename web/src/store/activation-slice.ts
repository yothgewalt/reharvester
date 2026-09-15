import type { StateCreator } from "zustand";

import { ApiError, transport } from "@/lib/api/transport";

import type { AppState } from "./index";
import { useProjectsStore } from "./projects";

const POLL_MS = 1000;

let activationSeq = 0;

export interface ActivationSlice {
  activation: { id: string | null; state: "idle" | "loading" | "error"; message: string };
  /**
   * Makes `id` the active corpus and reloads every view derived from the old
   * one. Never rejects; failures land in `activation`. A later call cancels an
   * earlier one still polling.
   */
  activateProject(id: string): Promise<void>;
}

/** The message to show for a failed activation request. */
export function activationErrorMessage(err: unknown): string {
  if (err instanceof ApiError) {
    const msg = err.fieldErrors?.["_"];
    if (msg) return msg;
    if (err.status === 0) return "Cannot reach the local API.";
  }
  return "This project couldn't be loaded.";
}

const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms));

export const createActivationSlice: StateCreator<AppState, [], [], ActivationSlice> = (set, get) => ({
  activation: { id: null, state: "idle", message: "" },

  activateProject: async (id) => {
    const seq = ++activationSeq;
    set({ activation: { id, state: "loading", message: "" } });
    try {
      let status = await transport.activateProject(id);
      const switched = status.status !== "active";
      while (status.status === "loading") {
        await sleep(POLL_MS);
        if (seq !== activationSeq) return;
        status = await transport.getActivation(id);
      }
      if (seq !== activationSeq) return;

      if (status.status !== "active") {
        const message =
          status.status === "inactive"
            ? "Another project was opened while this one was loading."
            : status.message || "This project couldn't be loaded.";
        set({ activation: { id, state: "error", message } });
        return;
      }

      if (switched) {
        get().resetCorpusView();
        set({ gapReport: null, emergingKeywords: [], wikiDoc: null });
        void useProjectsStore.getState().hydrate();
      }
      set({ activation: { id, state: "idle", message: "" } });
      if (switched || get().nodes.length === 0) void get().loadGraphSnapshot();
      void get().loadCommunities(switched);
    } catch (err) {
      if (seq !== activationSeq) return;
      set({ activation: { id, state: "error", message: activationErrorMessage(err) } });
    }
  },
});
