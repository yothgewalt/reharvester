/** Run with: bun test src/store/chat.test.ts */
import { expect, test } from "bun:test";

import { ApiError } from "@/lib/api/transport";

import { chatErrorMessage } from "./chat";

test("uses the server's message from a 409 with no active project", () => {
  const err = new ApiError(409, "409 Conflict", "/api/v1/chat/stream", {
    errors: { _: "No project is loaded — open one from Projects" },
  });
  expect(chatErrorMessage(err)).toBe("No project is loaded — open one from Projects");
});

test("uses the server's per-field message from a 400", () => {
  const err = new ApiError(400, "400 Bad Request", "/api/v1/chat/stream", {
    errors: { messages: "last message must be from the user" },
  });
  expect(chatErrorMessage(err)).toBe("last message must be from the user");
});

test("explains an unreachable API", () => {
  expect(chatErrorMessage(new ApiError(0, "Failed to fetch", "/api/v1/chat/stream"))).toBe(
    "Cannot reach the local API.",
  );
});

test("falls back to a generic message", () => {
  expect(chatErrorMessage(new Error("boom"))).toBe("boom");
  expect(chatErrorMessage("nope")).toBe("Something went wrong.");
});
