"use client";
import { useSearchParams } from "next/navigation";
import { useEffect, useRef } from "react";

import { useEnsureGraphSnapshot } from "@/lib/useEnsureGraphSnapshot";
import { useAppStore } from "@/store";

import { AskRail } from "./AskRail";
import { CommunityRail } from "./CommunityRail";
import { PaperReader } from "./PaperReader";

/**
 * Three panes over one corpus: communities, the selected paper, and the ask
 * surface. Layout-agnostic — it fills its container, so it works both as the
 * /reader page and as a tab inside a project.
 */
export function ReaderWorkbench() {
  const params = useSearchParams();
  const loadCommunities = useAppStore((s) => s.loadCommunities);
  const selectCommunity = useAppStore((s) => s.selectCommunity);
  const selectNode = useAppStore((s) => s.selectNode);
  const selectedDocId = useAppStore((s) => s.selectedDocId);
  const communities = useAppStore((s) => s.communities);

  useEnsureGraphSnapshot();

  useEffect(() => {
    void loadCommunities();
  }, [loadCommunities]);

  // Apply ?doc / ?c once, after communities land so a slug can resolve.
  const applied = useRef(false);
  useEffect(() => {
    if (applied.current || communities.length === 0) return;
    applied.current = true;
    const doc = params.get("doc");
    const community = params.get("c");
    if (community) selectCommunity(community);
    if (doc) selectNode(doc, doc);
  }, [params, communities, selectCommunity, selectNode]);

  // Keep the URL shareable without going through the router, which would
  // re-render the tree on every selection.
  useEffect(() => {
    if (!selectedDocId) return;
    const url = new URL(window.location.href);
    url.searchParams.set("doc", selectedDocId);
    window.history.replaceState(null, "", url);
  }, [selectedDocId]);

  return (
    <div className="grid min-h-0 grid-cols-1 overflow-hidden rounded-xl bg-white ring-line lg:grid-cols-[260px_1px_minmax(0,1fr)_1px_340px]">
      <nav aria-label="Communities" className="min-h-0 max-h-[720px] overflow-y-auto p-4">
        <CommunityRail />
      </nav>
      <div className="hidden bg-line lg:block" />
      <article
        aria-labelledby="reader-title"
        className="min-h-0 max-h-[720px] overflow-y-auto border-t border-line lg:border-t-0"
      >
        <PaperReader />
      </article>
      <div className="hidden bg-line lg:block" />
      <section
        aria-labelledby="ask-heading"
        className="min-h-0 max-h-[720px] overflow-y-auto border-t border-line lg:border-t-0"
      >
        <AskRail />
      </section>
    </div>
  );
}
