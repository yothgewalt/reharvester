"use client";
import { SectionShell } from "@/components/layout/SectionShell";

import { ReaderWorkbench } from "./ReaderWorkbench";

export function ReaderSection() {
  return (
    <SectionShell
      id="reader"
      eyebrow="Unified local corpus"
      title="Literature reader"
      description="Browse by community, follow the semantic backbone, and ask the corpus a question — all against the local index."
      tone="white"
    >
      <ReaderWorkbench />
    </SectionShell>
  );
}
