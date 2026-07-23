import type { Metadata } from "next";

import { CorpusSection } from "@/components/corpus/CorpusSection";

export const metadata: Metadata = { title: "Corpus — Reharvester" };

export default function CorpusPage() {
  return <CorpusSection />;
}
