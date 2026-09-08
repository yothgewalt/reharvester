import { create } from "zustand";

import { createAskSlice, type AskSlice } from "./ask-slice";
import { createCrawlSlice, type CrawlSlice } from "./crawl-slice";
import { createGraphSlice, type GraphSlice } from "./graph-slice";
import { createSchedulerSlice, type SchedulerSlice } from "./scheduler-slice";
import { createSystemSlice, type SystemSlice } from "./system-slice";
import { createTrendsSlice, type TrendsSlice } from "./trends-slice";
import { createWikiSlice, type WikiSlice } from "./wiki-slice";

export type AppState = SystemSlice &
  CrawlSlice &
  GraphSlice &
  WikiSlice &
  TrendsSlice &
  SchedulerSlice &
  AskSlice;

export const useAppStore = create<AppState>()((...a) => ({
  ...createSystemSlice(...a),
  ...createCrawlSlice(...a),
  ...createGraphSlice(...a),
  ...createWikiSlice(...a),
  ...createTrendsSlice(...a),
  ...createSchedulerSlice(...a),
  ...createAskSlice(...a),
}));
