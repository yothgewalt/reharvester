import type { Metadata } from "next";
import { Suspense } from "react";

import { GraphSection } from "@/components/graph/GraphSection";

export const metadata: Metadata = { title: "Graph — Reharvester" };

/**
 * The focused node travels in the query string (`?node=`); the Suspense
 * boundary is required because GraphSection reads useSearchParams.
 */
export default function GraphPage() {
  return (
    <Suspense fallback={null}>
      <GraphSection />
    </Suspense>
  );
}
