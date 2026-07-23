import type { Metadata } from "next";

import GraphSection from "@/components/graph/GraphSection";

export const metadata: Metadata = { title: "Graph — Reharvester" };

export default function GraphPage() {
  return <GraphSection />;
}
