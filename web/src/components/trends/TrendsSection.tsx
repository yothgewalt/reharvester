"use client";
import { useEffect } from "react";

import { SectionShell } from "@/components/layout/SectionShell";
import { usePrefersReducedMotion } from "@/components/layout/usePrefersReducedMotion";
import { useAppStore } from "@/store";

import { FieldMap } from "./FieldMap";
import { GapPairs } from "./GapPairs";
import { TimeWindowSlider } from "./TimeWindowSlider";
import { TrendCard } from "./TrendCard";
import { TrendDeltaChart } from "./TrendDeltaChart";

export function TrendsSection() {
  const emergingKeywords = useAppStore((s) => s.emergingKeywords);
  const trendsLoading = useAppStore((s) => s.trendsLoading);
  const recalculateTrends = useAppStore((s) => s.recalculateTrends);
  const loadCommunities = useAppStore((s) => s.loadCommunities);
  const loadGapPairs = useAppStore((s) => s.loadGapPairs);
  const prefersReducedMotion = usePrefersReducedMotion();

  useEffect(() => {
    if (useAppStore.getState().emergingKeywords.length === 0) {
      void recalculateTrends();
    }
  }, [recalculateTrends]);

  useEffect(() => {
    void loadCommunities();
    void loadGapPairs();
  }, [loadCommunities, loadGapPairs]);

  // The endpoint now carries both arms of Fig. 2a; the headline cards are the
  // risers only.
  const headline = emergingKeywords.filter((k) => k.direction === "rising").slice(0, 5);

  return (
    <SectionShell
      id="trends"
      eyebrow="Embedded kinetics"
      title="Top growing keywords"
      description="Smoothed log₂ prevalence lift against the preceding window, after filtering out vocabulary spread evenly across communities."
      tone="faint"
      headerAside={<TimeWindowSlider />}
    >
      <div aria-busy={trendsLoading} className="grid grid-cols-2 gap-6 lg:grid-cols-5">
        {headline.length === 0
          ? Array.from({ length: 5 }, (_, i) => (
              <div key={i} className="h-32 animate-pulse rounded-xl bg-white ring-line" />
            ))
          : headline.map((trend, index) => (
              <TrendCard
                key={trend.keyword}
                trend={trend}
                index={index}
                animate={!prefersReducedMotion}
              />
            ))}
      </div>

      <section aria-labelledby="delta-heading" className="flex flex-col gap-4 pt-4">
        <div className="flex flex-col gap-1">
          <span id="delta-heading" className="font-mono text-[13px] uppercase text-accent">
            Rising and declining vocabulary
          </span>
          <p className="m-0 max-w-[70ch] text-[14px] leading-6 text-ink-2">
            A decline is a finding too: vocabulary that receded tells you what the field
            moved away from, and a phrase that fell to zero is the sharpest signal of all.
          </p>
        </div>
        <TrendDeltaChart />
      </section>

      <section aria-labelledby="fieldmap-heading" className="flex flex-col gap-4 pt-4">
        <div className="flex flex-col gap-1">
          <span id="fieldmap-heading" className="font-mono text-[13px] uppercase text-accent">
            Coarse-grained field map
          </span>
          <p className="m-0 max-w-[70ch] text-[14px] leading-6 text-ink-2">
            The backbone aggregated to communities: what the corpus is made of, and which
            topics link to which.
          </p>
        </div>
        <FieldMap />
      </section>

      <section aria-labelledby="gaps-heading" className="flex flex-col gap-4 pt-4">
        <div className="flex flex-col gap-1">
          <span id="gaps-heading" className="font-mono text-[13px] uppercase text-accent">
            Research-gap candidates
          </span>
          <p className="m-0 max-w-[70ch] text-[14px] leading-6 text-ink-2">
            Pairs of communities whose documents read alike but between which few backbone
            links exist — topics that share vocabulary and methods but publish apart.
          </p>
        </div>
        <GapPairs />
      </section>
    </SectionShell>
  );
}
