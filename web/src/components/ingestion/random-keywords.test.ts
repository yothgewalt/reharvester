/** Run with: bun test src/components/ingestion/random-keywords.test.ts */
import { expect, test } from "bun:test";

import { randomKeywords, TOPICS } from "./random-keywords";
import { classifyIngestInput } from "./validate";

test("every local topic is a ready-to-crawl keyword set", () => {
  for (const topic of TOPICS) {
    const result = classifyIngestInput(topic.keywords.join(", "));
    expect({ field: topic.field, mode: result.mode, valid: result.valid }).toEqual({
      field: topic.field,
      mode: "keywords",
      valid: true,
    });
  }
});

test("without a model it returns the picked topic's local keywords", async () => {
  expect(await randomKeywords(false, () => 0)).toEqual([...TOPICS[0].keywords]);
  expect(await randomKeywords(false, () => 0.9999)).toEqual([...TOPICS[TOPICS.length - 1].keywords]);
});
