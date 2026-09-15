/** Run with: bun test src/store/activation-slice.test.ts */
import { expect, test } from "bun:test";

import { ApiError } from "@/lib/api/transport";

import { activationErrorMessage } from "./activation-slice";

test("uses the server's message from a 4xx errors body", () => {
  const err = new ApiError(409, "409 Conflict", "/x", { errors: { _: "This project has no corpus yet" } });
  expect(activationErrorMessage(err)).toBe("This project has no corpus yet");
});

test("explains an unreachable API", () => {
  expect(activationErrorMessage(new ApiError(0, "Failed to fetch", "/x"))).toBe("Cannot reach the local API.");
});

test("falls back to a generic message", () => {
  expect(activationErrorMessage(new ApiError(500, "500", "/x"))).toBe("This project couldn't be loaded.");
  expect(activationErrorMessage(new Error("boom"))).toBe("This project couldn't be loaded.");
});
