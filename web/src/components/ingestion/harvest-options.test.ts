/** Run with: bun test src/components/ingestion/harvest-options.test.ts */
import { expect, test } from "bun:test";

import type { HarvestDefaults } from "@/types/domain";

import { effectiveOptions, summarizeOptions, toRequestFields, validateOptions } from "./harvest-options";

const DEFAULTS: HarvestDefaults = { source: "arxiv", from: 2019, to: 2026, max: 2000 };
const CTX = { sources: ["arxiv", "auto", "kaggle", "semanticscholar", "openalex", "oai"], hasSnapshotPath: false };

test("effectiveOptions merges defaults with untouched fields following settings", () => {
  expect(effectiveOptions(DEFAULTS, {})).toEqual({
    name: undefined,
    source: "arxiv",
    categories: undefined,
    from: 2019,
    to: 2026,
    max: 2000,
  });
  expect(effectiveOptions(DEFAULTS, { source: "kaggle", max: 500 })).toEqual({
    name: undefined,
    source: "kaggle",
    categories: undefined,
    from: 2019,
    to: 2026,
    max: 500,
  });
});

test("summarizeOptions formats source, year range and max", () => {
  expect(summarizeOptions(effectiveOptions(DEFAULTS, {}))).toBe("arxiv · 2019–2026 · 2000");
});

test("summarizeOptions appends categories when present", () => {
  const opts = effectiveOptions(DEFAULTS, { categories: ["cs.LG", "cs.AI"] });
  expect(summarizeOptions(opts)).toBe("arxiv · 2019–2026 · 2000 · cs.LG, cs.AI");
});

test("validateOptions accepts the plain defaults", () => {
  expect(validateOptions(effectiveOptions(DEFAULTS, {}), CTX)).toEqual({});
});

test("validateOptions rejects a name over 120 characters", () => {
  const opts = effectiveOptions(DEFAULTS, { name: "x".repeat(121) });
  expect(validateOptions(opts, CTX).name).toBeDefined();
});

test("validateOptions rejects a source outside the known list", () => {
  const opts = effectiveOptions(DEFAULTS, { source: "carrier-pigeon" });
  expect(validateOptions(opts, CTX).source).toBeDefined();
});

test("validateOptions blocks kaggle without a snapshot path", () => {
  const opts = effectiveOptions(DEFAULTS, { source: "kaggle" });
  expect(validateOptions(opts, CTX).source).toBe("Set the arXiv snapshot path in Settings first");
});

test("validateOptions allows kaggle once a snapshot path is set", () => {
  const opts = effectiveOptions(DEFAULTS, { source: "kaggle" });
  expect(validateOptions(opts, { ...CTX, hasSnapshotPath: true }).source).toBeUndefined();
});

test("validateOptions rejects a year outside 1990-2100", () => {
  expect(validateOptions(effectiveOptions(DEFAULTS, { from: 1900 }), CTX).from).toBeDefined();
  expect(validateOptions(effectiveOptions(DEFAULTS, { to: 2200 }), CTX).to).toBeDefined();
});

test("validateOptions rejects from after to", () => {
  const opts = effectiveOptions(DEFAULTS, { from: 2025, to: 2020 });
  expect(validateOptions(opts, CTX).to).toBeDefined();
});

test("validateOptions rejects max outside 1-50000", () => {
  expect(validateOptions(effectiveOptions(DEFAULTS, { max: 0 }), CTX).max).toBeDefined();
  expect(validateOptions(effectiveOptions(DEFAULTS, { max: 50001 }), CTX).max).toBeDefined();
});

test("validateOptions rejects more than 16 categories", () => {
  const categories = Array.from({ length: 17 }, (_, i) => `cs.${i}`);
  expect(validateOptions(effectiveOptions(DEFAULTS, { categories }), CTX).categories).toBeDefined();
});

test("validateOptions rejects a category containing a space", () => {
  const opts = effectiveOptions(DEFAULTS, { categories: ["cs LG"] });
  expect(validateOptions(opts, CTX).categories).toBeDefined();
});

test("validateOptions requires categories for oai", () => {
  const opts = effectiveOptions(DEFAULTS, { source: "oai" });
  expect(validateOptions(opts, CTX).categories).toBeDefined();
});

test("validateOptions accepts oai with categories", () => {
  const opts = effectiveOptions(DEFAULTS, { source: "oai", categories: ["oai:arXiv.org:cs"] });
  expect(validateOptions(opts, CTX).categories).toBeUndefined();
});

test("toRequestFields omits untouched and empty fields", () => {
  expect(toRequestFields({})).toEqual({});
  expect(toRequestFields({ name: "", categories: [], from: 2020 })).toEqual({ from: 2020 });
  expect(toRequestFields({ name: "  Custom run  ", source: "arxiv", categories: ["cs.LG"], max: 500 })).toEqual({
    name: "Custom run",
    source: "arxiv",
    categories: ["cs.LG"],
    max: 500,
  });
});
