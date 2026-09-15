"use client";
import Chip from "@mui/material/Chip";
import Link from "next/link";

import { MathText } from "@/components/reader/MathText";
import type { ChatSource } from "@/types/domain";

/** Numbered source cards for one assistant message; `[n]` citations in that
 *  message link to `#src-<msgId>-n`, matching the ids rendered here. */
export function SourceList({ msgId, sources }: { msgId: string; sources: ChatSource[] }) {
  if (sources.length === 0) return null;
  const headingId = `sources-${msgId}`;
  return (
    <section aria-labelledby={headingId} className="mt-3 flex flex-col gap-2 border-t border-line pt-4">
      <h4
        id={headingId}
        className="m-0 font-mono text-[12px] font-normal tracking-wider text-ink-3 uppercase"
      >
        Sources · {sources.length}
      </h4>
      <ol aria-label="Sources" className="m-0 flex list-none flex-col gap-2 p-0">
        {sources.map((s, i) => {
          const n = i + 1;
          return (
            <li
              key={`${s.docId}-${i}`}
              id={`src-${msgId}-${n}`}
              tabIndex={-1}
              className="flex flex-wrap items-center gap-2 rounded-md p-2 text-[13px] ring-line"
            >
              <span className="font-mono text-[11px] text-ink-3">[{n}]</span>
              <span className="min-w-0 flex-1 text-ink-1">
                <MathText text={s.title} />
              </span>
              {s.viaGraph ? <Chip size="small" variant="outlined" label="via graph" /> : null}
              {s.docId ? (
                <Link
                  href={`/reader?doc=${encodeURIComponent(s.docId)}`}
                  className="shrink-0 text-[13px] text-accent underline underline-offset-2"
                >
                  Open in Reader
                </Link>
              ) : null}
            </li>
          );
        })}
      </ol>
    </section>
  );
}
