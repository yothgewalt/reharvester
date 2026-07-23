import type { Metadata } from "next";

import { TrendsSection } from "@/components/trends/TrendsSection";

export const metadata: Metadata = { title: "Trends — Reharvester" };

export default function TrendsPage() {
  return <TrendsSection />;
}
