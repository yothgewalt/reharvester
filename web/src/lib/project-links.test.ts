/** Run with: bun test src/lib/project-links.test.ts */
import { expect, test } from "bun:test";

import { projectChatHref } from "./project-links";

test("links to a project's chat tab, optionally pinned and encoded", () => {
  expect(projectChatHref("dark-matter-2022")).toBe("/projects/detail?id=dark-matter-2022&tab=chat");
  expect(projectChatHref("p1", "a paper/with?odd&chars")).toBe(
    "/projects/detail?id=p1&tab=chat&pin=a+paper%2Fwith%3Fodd%26chars",
  );
});
