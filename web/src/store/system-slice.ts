import type { StateCreator } from "zustand";

import type { AppState } from "./index";

export type BackendStatus = "checking" | "online" | "offline" | "mocked";
export type LlmStatus = "unknown" | "ok" | "unreachable";

export interface SystemSlice {
  backendStatus: BackendStatus;
  llmStatus: LlmStatus;
  webglSupported: boolean | null;
  setBackendStatus(s: BackendStatus): void;
  setLlmStatus(s: LlmStatus): void;
  setWebglSupported(ok: boolean): void;
}

export const createSystemSlice: StateCreator<AppState, [], [], SystemSlice> = (set) => ({
  backendStatus: "checking",
  llmStatus: "unknown",
  webglSupported: null,
  setBackendStatus: (backendStatus) => set({ backendStatus }),
  setLlmStatus: (llmStatus) => set({ llmStatus }),
  setWebglSupported: (webglSupported) => set({ webglSupported }),
});
