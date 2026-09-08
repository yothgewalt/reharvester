import type { StateCreator } from "zustand";

import type { AppState } from "./index";

import type { CapabilityTier } from "@/types/domain";

export type BackendStatus = "checking" | "online" | "offline";
export type LlmStatus = "unknown" | "ok" | "unreachable";

export interface SystemSlice {
  backendStatus: BackendStatus;
  llmStatus: LlmStatus;
  capabilityTier: CapabilityTier | null;
  setBackendStatus(s: BackendStatus): void;
  setLlmStatus(s: LlmStatus): void;
  setCapabilityTier(t: CapabilityTier | null): void;
}

export const createSystemSlice: StateCreator<AppState, [], [], SystemSlice> = (set) => ({
  backendStatus: "checking",
  llmStatus: "unknown",
  capabilityTier: null,
  setBackendStatus: (backendStatus) => set({ backendStatus }),
  setLlmStatus: (llmStatus) => set({ llmStatus }),
  setCapabilityTier: (capabilityTier) => set({ capabilityTier }),
});
