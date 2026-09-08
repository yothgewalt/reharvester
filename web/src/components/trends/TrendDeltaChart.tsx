"use client";
import Typography from "@mui/material/Typography";
import { useMemo } from "react";

import { useAppStore } from "@/store";
import type { TrendKeyword } from "@/types/domain";

/**
 * Diverging bars — paper Fig. 2a. Seven fastest-rising and seven steepest-
 * declining bigrams by smoothed log2 prevalence lift.
 *
 * Colour is the validated diverging pair, but it is redundant: the side of the
 * zero axis and the ▲/▼ glyph both carry direction, so the chart survives
 * colour-blindness, greyscale print and forced-colors.
 */
const RISE = "#2a78d6";
const FALL = "#e34948";

export function TrendDeltaChart() {
  const keywords = useAppStore((s) => s.emergingKeywords);
  const loading = useAppStore((s) => s.trendsLoading);

  const { rising, declining, max } = useMemo(() => {
    const rising = keywords.filter((k) => k.direction === "rising");
    const declining = keywords.filter((k) => k.direction === "declining");
    const max = Math.max(1, ...keywords.map((k) => Math.abs(k.growthRate)));
    return { rising, declining, max };
  }, [keywords]);

  if (keywords.length === 0) {
    return (
      <div className="h-40 animate-pulse rounded-xl bg-white ring-line" aria-hidden="true" />
    );
  }

  return (
    <figure className="m-0 flex flex-col gap-3" aria-busy={loading}>
      <div className="flex flex-col gap-4 rounded-xl bg-white p-5 ring-line">
        <Arm rows={rising} max={max} color={RISE} sign="▲" label="Rising" />
        <div className="h-px w-full bg-line" role="presentation" />
        <Arm rows={declining} max={max} color={FALL} sign="▼" label="Declining" />
      </div>

      <figcaption className="m-0 font-mono text-[11px] leading-4 text-ink-3">
        Smoothed log₂ prevalence lift (α = 0.5) between the selected window and the equal-length
        one before it, after the specificity filter (S ≤ 0.80). Counts are documents,
        early → late; a lift compares shares, so on unequal windows a raw count can move the
        other way.
      </figcaption>
    </figure>
  );
}

function Arm({
  rows,
  max,
  color,
  sign,
  label,
}: {
  rows: TrendKeyword[];
  max: number;
  color: string;
  sign: string;
  label: string;
}) {
  if (rows.length === 0) {
    return (
      <Typography variant="caption" className="text-ink-2">
        No {label.toLowerCase()} terms in this window.
      </Typography>
    );
  }
  return (
    <section aria-label={`${label} terms`} className="flex flex-col gap-1.5">
      <Typography variant="overline" className="text-ink-3">
        {label}
      </Typography>
      <ul className="m-0 flex list-none flex-col gap-1 p-0">
        {rows.map((k) => (
          <li key={k.keyword} className="grid grid-cols-[minmax(0,11rem)_1fr_auto] items-center gap-3">
            <span className="truncate text-[13px] text-ink-1" title={k.keyword}>
              {k.keyword}
            </span>
            <span className="flex h-2 items-center" aria-hidden="true">
              <span
                className="h-2 rounded-sm"
                style={{
                  width: `${(Math.abs(k.growthRate) / max) * 100}%`,
                  background: color,
                }}
              />
            </span>
            <span className="whitespace-nowrap font-mono text-[11px] text-ink-3">
              <span aria-hidden="true">{sign}</span>{" "}
              <span className="sr-only">{label},</span>
              {k.growthRate > 0 ? "+" : ""}
              {k.growthRate.toFixed(2)} log₂
              <span className="text-ink-4"> · </span>
              {k.earlyDocs} <span className="sr-only">to</span>
              <span aria-hidden="true">→</span> {k.count} docs
            </span>
          </li>
        ))}
      </ul>
    </section>
  );
}
