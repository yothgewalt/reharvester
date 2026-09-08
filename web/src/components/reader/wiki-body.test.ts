/**
 * Run with: bun test src/components/reader/wiki-body.test.ts
 * Guards the coupling to rag.Render's line formats (internal/rag/wiki.go).
 */
import { expect, test } from "bun:test";

import { linkifyConcepts, parseWikiBody } from "./wiki-body";

const RENDERED = `# Dense Passage Retrieval for Open-Domain Question Answering

Karpukhin, Oguz, Min, Lewis · 2020

Open-domain question answering relies on efficient passage retrieval. We show
retrieval can be implemented using dense representations alone.

Community *llms language model* · PageRank 0.0031 · bridge score 0.41

## Nearest neighbours

Along the semantic backbone:

- [[sentence-bert|Sentence-BERT]] · cos 0.68
`;

test("extracts byline and prose, dropping heading, metrics and neighbours", () => {
  const { byline, prose } = parseWikiBody(RENDERED);
  expect(byline).toBe("Karpukhin et al.");
  expect(prose).toContain("Open-domain question answering");
  expect(prose).not.toContain("PageRank");
  expect(prose).not.toContain("Nearest neighbours");
  expect(prose).not.toContain("#");
});

test("a single author keeps its surname", () => {
  expect(parseWikiBody("# T\n\nKarpukhin · 2020\n\nBody.\n").byline).toBe("Karpukhin");
});

test("markdown without the expected anchors survives unchanged", () => {
  const raw = "Just some prose with no byline and no metrics line.";
  const { byline, prose } = parseWikiBody(raw);
  expect(byline).toBe("");
  expect(prose).toBe(raw);
});

test("linkify wraps each concept once only", () => {
  const out = linkifyConcepts(
    "We use dense representations. Dense representations beat BM25.",
    ["dense representations", "BM25"],
  );
  expect(out).toContain("[[dense representations]]");
  expect(out).toContain("[[BM25]]");
  expect(out).not.toContain("[[[[");
  expect(out.match(/\[\[/g)?.length).toBe(2);
});

test("linkify skips headings, list items and fenced spans", () => {
  const out = linkifyConcepts("- BM25 in a list\n# BM25 heading\n`BM25` in code", ["BM25"]);
  expect(out).not.toContain("[[BM25]]");
});
