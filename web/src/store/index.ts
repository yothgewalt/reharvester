import { create } from "zustand";

import { createActivationSlice, type ActivationSlice } from "./activation-slice";
import { createCrawlSlice, type CrawlSlice } from "./crawl-slice";
import { createGraphSlice, type GraphSlice } from "./graph-slice";
import { createSchedulerSlice, type SchedulerSlice } from "./scheduler-slice";
import { createSettingsSlice, type SettingsSlice } from "./settings-slice";
import { createSystemSlice, type SystemSlice } from "./system-slice";
import { createTrendsSlice, type TrendsSlice } from "./trends-slice";
import { createWikiSlice, type WikiSlice } from "./wiki-slice";

export type AppState = SystemSlice &
  CrawlSlice &
  GraphSlice &
  WikiSlice &
  TrendsSlice &
  SchedulerSlice &
  SettingsSlice &
  ActivationSlice;

export const useAppStore = create<AppState>()((...a) => ({
  ...createActivationSlice(...a),
  ...createSystemSlice(...a),
  ...createCrawlSlice(...a),
  ...createGraphSlice(...a),
  ...createWikiSlice(...a),
  ...createTrendsSlice(...a),
  ...createSchedulerSlice(...a),
  ...createSettingsSlice(...a),
}));
