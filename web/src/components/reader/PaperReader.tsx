"use client";
import Typography from "@mui/material/Typography";
import { useMemo } from "react";

import { WikiPane } from "@/components/corpus/WikiPane";
import { useAppStore } from "@/store";

import { linkifyConcepts, parseWikiBody } from "./wiki-body";

const NEIGHBOUR_CAP = 12;

export function PaperReader() {
  const nodes = useAppStore((s) => s.nodes);
  const edges = useAppStore((s) => s.edges);
  const selectedDocId = useAppStore((s) => s.selectedDocId);
  const selectNode = useAppStore((s) => s.selectNode);
  const wikiDoc = useAppStore((s) => s.wikiDoc);

  const node = useMemo(
    () => nodes.find((n) => n.data.docId === selectedDocId) ?? null,
    [nodes, selectedDocId],
  );

  const conceptLabels = useMemo(
    () => nodes.filter((n) => n.data.kind === "concept").map((n) => n.label),
    [nodes],
  );

  const body = useMemo(() => {
    if (!wikiDoc?.markdown) return null;
    const { byline, prose } = parseWikiBody(wikiDoc.markdown);
    return { byline, prose: linkifyConcepts(prose, conceptLabels) };
  }, [wikiDoc, conceptLabels]);

  const labelById = useMemo(() => new Map(nodes.map((n) => [n.id, n.label])), [nodes]);

  // Backbone neighbours only: "co-authored-with" and "appears-in" carry weight 1
  // and would otherwise sort to the top as a perfect cosine.
  const neighbours = useMemo(() => {
    if (!selectedDocId) return [];
    const out: Array<{ id: string; label: string; cos: number }> = [];
    for (const e of edges) {
      if (e.relation !== "similar-to") continue;
      const other = e.source === selectedDocId ? e.target : e.target === selectedDocId ? e.source : null;
      if (!other) continue;
      const label = labelById.get(other);
      if (label) out.push({ id: other, label, cos: e.weight });
    }
    return out.sort((a, b) => b.cos - a.cos).slice(0, NEIGHBOUR_CAP);
  }, [edges, selectedDocId, labelById]);

  if (!node) {
    return (
      <div className="flex h-full items-center justify-center p-6">
        <Typography variant="caption" className="text-ink-2">
          Select a community or search for a paper to open it here.
        </Typography>
      </div>
    );
  }

  const isPaper = node.data.kind === "paper";
  const meta = [
    body?.byline,
    isPaper && node.data.year ? String(node.data.year) : "",
    node.data.category,
    // A concept node has no edges of its own, so its bridge score is a
    // meaningless 0.00 rather than a real structural measure.
    isPaper ? `bridge score ${node.data.bridge.toFixed(2)}` : "",
    node.data.gap ? "research gap" : "",
  ].filter(Boolean);

  return (
    <div className="flex flex-col gap-6 p-6">
      <header className="flex flex-col gap-2">
        <Typography variant="h5" component="h1" id="reader-title" tabIndex={-1}>
          {node.label}
        </Typography>
        <p className="m-0 min-h-5 font-mono text-[13px] text-ink-3">{meta.join(" · ")}</p>
      </header>

      <WikiPane markdown={body?.prose} />

      {neighbours.length > 0 ? (
        <section aria-labelledby="neighbours-heading" className="flex flex-col gap-2">
          <Typography variant="overline" id="neighbours-heading" className="text-ink-3">
            Neighbours along the backbone
          </Typography>
          <ul className="m-0 flex list-none flex-col gap-px p-0">
            {neighbours.map((n) => (
              <li key={n.id} className="contents">
                <button
                  type="button"
                  onClick={() => selectNode(n.id, n.id)}
                  className="flex w-full items-center justify-between gap-4 rounded-md px-3 py-2 text-left text-[14px] text-ink-1 transition-colors hover:bg-bg-subtle"
                >
                  <span className="truncate">{n.label}</span>
                  <span className="shrink-0 font-mono text-[12px] text-accent">
                    <span className="sr-only">cosine similarity </span>
                    cos {n.cos.toFixed(2)}
                  </span>
                </button>
              </li>
            ))}
          </ul>
        </section>
      ) : null}
    </div>
  );
}
