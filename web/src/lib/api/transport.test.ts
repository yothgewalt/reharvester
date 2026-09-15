/** Run with: bun test src/lib/api/transport.test.ts */
import { expect, test } from "bun:test";

import { ApiError } from "./transport";

test("fieldErrors reads the {errors} shape of a 400 body", () => {
  const err = new ApiError(400, "400 Bad Request", "/api/v1/settings", { errors: { max: "must be 1..50000" } });
  expect(err.fieldErrors).toEqual({ max: "must be 1..50000" });
});

test("fieldErrors drops non-string entries", () => {
  const err = new ApiError(400, "400 Bad Request", "/api/v1/settings", { errors: { max: 42 } });
  expect(err.fieldErrors).toBeNull();
});

test("fieldErrors is null with no body", () => {
  const err = new ApiError(500, "500 Internal Server Error", "/health");
  expect(err.fieldErrors).toBeNull();
});

test("fieldErrors is null when the body has no errors key", () => {
  const err = new ApiError(400, "400 Bad Request", "/api/v1/settings", { message: "bad request" });
  expect(err.fieldErrors).toBeNull();
});
