/** Run with: bun test src/components/reader/MathText.test.ts */
import { expect, test } from "bun:test";

import { splitMath } from "./MathText";

test("splits inline TeX from surrounding text", () => {
  expect(splitMath("$\\mathcal{N}=1$ Jackiw-Teitelboim supergravity")).toEqual([
    { math: true, value: "\\mathcal{N}=1" },
    { math: false, value: " Jackiw-Teitelboim supergravity" },
  ]);
  expect(splitMath("a $x$ b $y$")).toEqual([
    { math: false, value: "a " },
    { math: true, value: "x" },
    { math: false, value: " b " },
    { math: true, value: "y" },
  ]);
});

test("text without a closed pair stays plain", () => {
  expect(splitMath("costs $5 per run")).toEqual([{ math: false, value: "costs $5 per run" }]);
  expect(splitMath("")).toEqual([]);
});
