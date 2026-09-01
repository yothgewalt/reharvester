import type { StateCreator } from "zustand";

import { transport } from "@/lib/api/transport";
import type { JobRegisterRequest, SchedulerProfile } from "@/types/domain";

import type { AppState } from "./index";

export interface SchedulerSlice {
  schedulerProfiles: SchedulerProfile[];
  schedulerSubmitting: boolean;
  schedulerError: string | null;

  registerProfile(req: JobRegisterRequest): Promise<boolean>;
  loadJobs(): Promise<void>;
}

export const createSchedulerSlice: StateCreator<AppState, [], [], SchedulerSlice> = (set) => ({
  schedulerProfiles: [],
  schedulerSubmitting: false,
  schedulerError: null,

  loadJobs: async () => {
    try {
      set({ schedulerProfiles: await transport.listJobs() });
    } catch {
      // A missing job list is not worth blocking the page over; the form still
      // works and the next registration repopulates the list.
    }
  },

  registerProfile: async (req) => {
    set({ schedulerSubmitting: true, schedulerError: null });
    try {
      const profile = await transport.registerJob(req);
      set((s) => ({
        schedulerProfiles: [...s.schedulerProfiles, profile],
        schedulerSubmitting: false,
      }));
      return true;
    } catch (err) {
      set({
        schedulerSubmitting: false,
        schedulerError: err instanceof Error ? err.message : "Registration failed",
      });
      return false;
    }
  },
});
