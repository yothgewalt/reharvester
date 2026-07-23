"use client";
import { useEffect } from "react";

import { SectionShell } from "@/components/layout/SectionShell";
import { usePrefersReducedMotion } from "@/components/layout/usePrefersReducedMotion";
import { useAppStore } from "@/store";

import { TimeWindowSlider } from "./TimeWindowSlider";
import { TrendCard } from "./TrendCard";

export function TrendsSection() {
  const emergingKeywords = useAppStore((s) => s.emergingKeywords);
  const trendsLoading = useAppStore((s) => s.trendsLoading);
  const recalculateTrends = useAppStore((s) => s.recalculateTrends);
  const prefersReducedMotion = usePrefersReducedMotion();

  useEffect(() => {
    if (useAppStore.getState().emergingKeywords.length === 0) {
      void recalculateTrends();
    }
  }, [recalculateTrends]);

  return (
    <SectionShell
      id="trends"
      eyebrow="Embedded kinetics"
      title="Top growing keywords"
      description="Year-over-year regression slope across the local corpus."
      tone="faint"
      headerAside={<TimeWindowSlider />}
    >
      <div aria-busy={trendsLoading} className="grid grid-cols-2 gap-6 lg:grid-cols-5">
        {emergingKeywords.length === 0
          ? Array.from({ length: 5 }, (_, i) => (
              <div key={i} className="h-32 animate-pulse rounded-xl bg-white ring-line" />
            ))
          : emergingKeywords.map((trend, index) => (
              <TrendCard
                key={trend.keyword}
                trend={trend}
                index={index}
                animate={!prefersReducedMotion}
              />
            ))}
      </div>
    </SectionShell>
  );
}
