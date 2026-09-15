import katex from "katex";
import { Fragment, useMemo } from "react";

const INLINE_MATH = /\$([^$\n]+)\$/g;

/** Splits plain text into text runs and inline `$…$` TeX runs, in order. */
export function splitMath(text: string): Array<{ math: boolean; value: string }> {
  const parts: Array<{ math: boolean; value: string }> = [];
  let last = 0;
  for (const m of text.matchAll(INLINE_MATH)) {
    if (m.index > last) parts.push({ math: false, value: text.slice(last, m.index) });
    parts.push({ math: true, value: m[1] });
    last = m.index + m[0].length;
  }
  if (last < text.length) parts.push({ math: false, value: text.slice(last) });
  return parts;
}

/**
 * Renders a plain-text string (a paper title, a neighbour label) with its
 * inline `$…$` TeX typeset by KaTeX. For markdown bodies use WikiPane instead.
 * Invalid TeX renders as its source rather than throwing.
 */
export function MathText({ text }: { text: string }) {
  const parts = useMemo(() => splitMath(text), [text]);
  return (
    <>
      {parts.map((p, i) =>
        p.math ? (
          <span
            key={i}
            dangerouslySetInnerHTML={{
              __html: katex.renderToString(p.value, { throwOnError: false, strict: "ignore" }),
            }}
          />
        ) : (
          <Fragment key={i}>{p.value}</Fragment>
        ),
      )}
    </>
  );
}
