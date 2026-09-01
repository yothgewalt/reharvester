"use client";
import Typography from "@mui/material/Typography";
import { useRouter } from "next/navigation";
import { useMemo } from "react";

import { useAppStore } from "@/store";
import type { Community, GapPair } from "@/types/domain";

import { displayLabel } from "./field-map-layout";

/** The paper reports the seven highest-similarity under-connected pairs. */
const SHOWN = 7;

/**
 * Research-gap candidates — paper Fig. 2d. Community pairs whose documents read
 * alike but between which few backbone links exist.
 *
 * The caveat is part of the result, not a disclaimer bolted on: when nearly
 * every pair clears z < -2, the permutation null is a weak filter and centroid
 * similarity is doing the ranking. The paper calls this a shortlist generator
 * rather than a hypothesis test, and this panel says so on screen.
 */
export function GapPairs() {
  const report = useAppStore((s) => s.gapReport);
  const communities = useAppStore((s) => s.communities);
  const selectCommunity = useAppStore((s) => s.selectCommunity);
  const router = useRouter();

  const byId = useMemo(() => new Map(communities.map((c) => [c.id, c])), [communities]);

  if (!report) {
    return <div className="h-40 animate-pulse rounded-xl bg-white ring-line" aria-hidden="true" />;
  }
  if (report.pairs.length === 0) {
    return (
      <Typography variant="caption" className="text-ink-2">
        No under-connected community pairs — the corpus may be too small to partition.
      </Typography>
    );
  }

  const weakFilter = report.total > 0 && report.belowThreshold / report.total > 0.5;

  const open = (c: Community | undefined) => {
    if (!c) return;
    selectCommunity(c.slug);
    router.push(`/reader?c=${encodeURIComponent(c.slug)}`);
  };

  return (
    <figure className="m-0 flex flex-col gap-3">
      <ul className="m-0 flex list-none flex-col gap-px overflow-hidden rounded-xl bg-white p-0 ring-line">
        {report.pairs.slice(0, SHOWN).map((p) => (
          <li
            key={`${p.a}-${p.b}`}
            className="grid grid-cols-1 items-center gap-2 border-b border-line p-4 last:border-b-0 sm:grid-cols-[1fr_auto]"
          >
            <span className="flex flex-wrap items-center gap-2 text-[14px] text-ink-1">
              <Side community={byId.get(p.a)} fallback={p.labelA} onOpen={open} />
              <span className="text-ink-3" aria-hidden="true">
                ↔
              </span>
              <Side community={byId.get(p.b)} fallback={p.labelB} onOpen={open} />
            </span>
            <span className="whitespace-nowrap font-mono text-[11px] text-ink-3">
              <span className="sr-only">centroid cosine </span>cos {p.similarity.toFixed(3)}
              <span className="text-ink-4"> · </span>
              {p.observed.toLocaleString()} vs{" "}
              <span className="sr-only">null mean </span>
              {Math.round(p.nullMean).toLocaleString()} links
              <span className="text-ink-4"> · </span>
              <span className="sr-only">z-score </span>z {p.z.toFixed(1)}
            </span>
          </li>
        ))}
      </ul>

      <figcaption className="m-0 font-mono text-[11px] leading-4 text-ink-3">
        Ranked by centroid cosine among pairs with z &lt; −2 against a 200-permutation null.
        {weakFilter ? (
          <>
            {" "}
            <strong className="font-normal text-ink-2">
              {report.belowThreshold.toLocaleString()} of {report.total.toLocaleString()} pairs
              clear that threshold
            </strong>
            , so the z-score is a weak filter here and similarity carries the ranking. Treat
            this as a shortlist to inspect, not a hypothesis test.
          </>
        ) : (
          <>
            {" "}
            {report.belowThreshold.toLocaleString()} of {report.total.toLocaleString()} pairs
            clear that threshold.
          </>
        )}
      </figcaption>
    </figure>
  );
}

function Side({
  community,
  fallback,
  onOpen,
}: {
  community: Community | undefined;
  fallback: string;
  onOpen(c: Community | undefined): void;
}) {
  const text = community ? displayLabel(community) : fallback;
  if (!community) return <span className="text-ink-2">{text}</span>;
  return (
    <button
      type="button"
      onClick={() => onOpen(community)}
      title={`${community.label} · ${community.size.toLocaleString()} papers`}
      className="rounded-md px-1.5 py-0.5 text-left text-ink-1 underline decoration-dotted underline-offset-2 transition-colors hover:bg-bg-subtle"
    >
      {text}
    </button>
  );
}

export type { GapPair };
