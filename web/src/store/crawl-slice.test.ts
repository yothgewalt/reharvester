/** Run with: bun test src/store/crawl-slice.test.ts */
import { expect, test } from "bun:test";

import type { CrawlLogEntry } from "@/types/domain";

import { harvestSucceeded } from "./crawl-slice";

const log = (level: CrawlLogEntry["level"]): CrawlLogEntry => ({
  id: level,
  timestamp: "2026-09-15T00:00:00Z",
  level,
  message: level,
});

test("a harvest succeeds only with ingested papers and no error lines", () => {
  expect(harvestSucceeded(820, [log("info"), log("warn"), log("success")])).toBe(true);
  expect(harvestSucceeded(0, [log("info"), log("warn")])).toBe(false);
  expect(harvestSucceeded(820, [log("info"), log("error")])).toBe(false);
});
