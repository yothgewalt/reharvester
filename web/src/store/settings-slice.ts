import type { StateCreator } from "zustand";

import { requestHealthPing } from "@/lib/api/failure-bus";
import { ApiError, transport } from "@/lib/api/transport";
import type { FieldErrors, SettingsPatch, SettingsView } from "@/types/domain";

import type { AppState } from "./index";

export type SaveState = "idle" | "saving" | "saved" | "error";

export interface SaveStatus {
  state: SaveState;
  message?: string;
}

const MODEL_FIELDS: ReadonlySet<string> = new Set(["ollamaUrl", "chatModel", "ollamaKey", "embedModel"]);

const errorMessage = (err: unknown): string => (err instanceof Error ? err.message : "unknown error");

export interface SettingsSlice {
  settings: SettingsView | null;
  settingsLoadError: string | null;
  saveStatus: SaveStatus;
  loadSettings(): Promise<void>;
  /** Runs one PATCH at a time; "saved" only shows once every queued save has landed. */
  saveSetting(patch: SettingsPatch): Promise<FieldErrors | null>;
}

export const createSettingsSlice: StateCreator<AppState, [], [], SettingsSlice> = (set) => {
  let queue: Promise<FieldErrors | null> = Promise.resolve(null);
  let pending = 0;

  const run = async (patch: SettingsPatch): Promise<FieldErrors | null> => {
    pending += 1;
    set({ saveStatus: { state: "saving" } });
    try {
      const settings = await transport.patchSettings(patch);
      set({ settings });
      if (Object.keys(patch).some((key) => MODEL_FIELDS.has(key))) requestHealthPing();
      pending -= 1;
      if (pending === 0) {
        const message = settings.notices.length > 0 ? settings.notices.join(" ") : undefined;
        set({ saveStatus: { state: "saved", message } });
      }
      return null;
    } catch (err) {
      pending -= 1;
      const fieldErrors = err instanceof ApiError ? err.fieldErrors : null;
      if (pending === 0) {
        set({
          saveStatus: {
            state: "error",
            message: fieldErrors ? "Couldn't save: fix the highlighted field." : `Couldn't save: ${errorMessage(err)}`,
          },
        });
      }
      return fieldErrors;
    }
  };

  return {
    settings: null,
    settingsLoadError: null,
    saveStatus: { state: "idle" },

    loadSettings: async () => {
      set({ settingsLoadError: null });
      try {
        const settings = await transport.getSettings();
        set({ settings });
      } catch (err) {
        set({ settingsLoadError: errorMessage(err) });
      }
    },

    saveSetting: (patch) => {
      const scheduled = queue.then(() => run(patch));
      queue = scheduled.catch(() => null);
      return scheduled;
    },
  };
};
