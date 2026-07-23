"use client";
import { SectionShell } from "@/components/layout/SectionShell";
import { useEnsureGraphSnapshot } from "@/lib/useEnsureGraphSnapshot";

import { PaperList } from "./PaperList";
import { WikiPane } from "./WikiPane";


export function CorpusBrowser() {


  useEnsureGraphSnapshot();
  return (
    <div className="grid grid-cols-[440px_1px_1fr] gap-0 overflow-hidden rounded-xl bg-white ring-line">
      <div className="p-5">
        <PaperList />
      </div>
      <div className="bg-line" />
      <div className="max-h-[640px] min-h-[560px] overflow-y-auto p-6">
        <WikiPane />
      </div>
    </div>
  );
}


export function CorpusSection() {
  return (
    <SectionShell
      id="corpus"
      eyebrow="Unified local corpus"
      title="Literature & wiki reader"
      tone="white"
    >
      <CorpusBrowser />
    </SectionShell>
  );
}
