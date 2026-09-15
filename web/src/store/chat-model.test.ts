/** Run with: bun test src/store/chat-model.test.ts */
import { expect, test } from "bun:test";

import type { ChatMessage } from "./chat-model";
import { historyFor, linkCitations, titleFrom } from "./chat-model";

const msg = (over: Partial<ChatMessage>): ChatMessage => ({
  id: over.id ?? "m",
  role: over.role ?? "user",
  content: over.content ?? "",
  status: over.status ?? "done",
  ...over,
});

test("titleFrom takes the first line and truncates long ones", () => {
  expect(titleFrom("Which research gaps look most promising?")).toBe(
    "Which research gaps look most promising?",
  );
  expect(titleFrom("first line\nsecond line")).toBe("first line");
  expect(titleFrom("   ")).toBe("New chat");
  const long = "a".repeat(60);
  const title = titleFrom(long);
  expect(title.length).toBe(48);
  expect(title.endsWith("…")).toBe(true);
});

test("historyFor keeps only finished, non-guard messages and caps at 12", () => {
  const messages: ChatMessage[] = [
    msg({ id: "1", role: "user", content: "q1" }),
    msg({ id: "2", role: "assistant", content: "a1" }),
    msg({ id: "3", role: "assistant", content: "refusal", guard: "execute" }),
    msg({ id: "4", role: "assistant", content: "still streaming", status: "streaming" }),
    msg({ id: "5", role: "assistant", content: "broke", status: "error" }),
    msg({ id: "6", role: "user", content: "q2" }),
  ];
  expect(historyFor(messages)).toEqual([
    { role: "user", content: "q1" },
    { role: "assistant", content: "a1" },
    { role: "user", content: "q2" },
  ]);

  const refusedThenAsked: ChatMessage[] = [
    msg({ id: "1", role: "user", content: "Run a harvest on quantum computing" }),
    msg({ id: "2", role: "assistant", content: "I can only answer…", guard: "execute" }),
    msg({ id: "3", role: "user", content: "What is the capital of France?" }),
  ];
  expect(historyFor(refusedThenAsked)).toEqual([{ role: "user", content: "What is the capital of France?" }]);

  const many = Array.from({ length: 20 }, (_, i) =>
    msg({ id: String(i), role: i % 2 === 0 ? "user" : "assistant", content: `m${i}` }),
  );
  expect(historyFor(many)).toHaveLength(12);
  expect(historyFor(many)[0]).toEqual({ role: "user", content: "m8" });
});

test("linkCitations only links numbers within range", () => {
  expect(linkCitations("See [1] and [2].", "msg-1", 2)).toBe(
    "See [1](#src-msg-1-1) and [2](#src-msg-1-2).",
  );
  expect(linkCitations("See [1] and [3].", "msg-1", 2)).toBe("See [1](#src-msg-1-1) and [3].");
  expect(linkCitations("No sources here [1].", "msg-1", 0)).toBe("No sources here [1].");
});
