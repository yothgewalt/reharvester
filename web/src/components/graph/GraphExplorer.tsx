"use client";
import Autocomplete from "@mui/material/Autocomplete";
import TextField from "@mui/material/TextField";
import ToggleButton from "@mui/material/ToggleButton";
import ToggleButtonGroup from "@mui/material/ToggleButtonGroup";
import { useSearchParams } from "next/navigation";
import { useEffect, useEffectEvent, useMemo, useRef, useState } from "react";

import { API_BASE } from "@/lib/config";
import { useEnsureGraphSnapshot } from "@/lib/useEnsureGraphSnapshot";
import { useAppStore } from "@/store";
import { color } from "@/theme/tokens";
import type { GraphNode, GraphRelation } from "@/types/domain";

import { GraphCanvas, type Hover } from "./GraphCanvas";
import { buildAdjacency, type Direction, RELATION_LABEL, RELATIONS, visibleLinks } from "./graph-model";
import { NodePanel } from "./NodePanel";

const MATCH_CAP = 50;
const SUMMARY_ID = "graph-summary";

const RELATION_TOGGLE: Record<GraphRelation, string> = {
  "similar-to": "Similar",
  "nearest-to": "Nearest",
  "co-authored-with": "Co-author",
  "appears-in": "Concept",
};

function matchNodes(nodes: readonly GraphNode[], input: string): GraphNode[] {
  const q = input.trim().toLowerCase();
  const hits = q ? nodes.filter((n) => n.label.toLowerCase().includes(q)) : nodes;
  return hits.slice(0, MATCH_CAP);
}

/**
 * The active project's snapshot as a navigable node-link diagram, without a
 * page shell; rendered by the project page's Graph tab once that project is
 * active, so it only ever shows that project's nodes and edges. Focus lives in `?node=`
 * (other query params are kept) so a view is shareable; it stays local rather
 * than going through the store's selectNode, which would also fetch the wiki
 * document. Render inside a Suspense boundary: it reads useSearchParams.
 */
export function GraphExplorer() {
  const params = useSearchParams();
  const nodes = useAppStore((s) => s.nodes);
  const edges = useAppStore((s) => s.edges);
  const graphLoading = useAppStore((s) => s.graphLoading);
  const graphError = useAppStore((s) => s.graphError);
  const backendStatus = useAppStore((s) => s.backendStatus);
  useEnsureGraphSnapshot();

  const [focusId, setFocusId] = useState<string | null>(() => params.get("node"));
  const [mode, setMode] = useState<Direction>("all");
  const [relations, setRelations] = useState<ReadonlySet<GraphRelation>>(() => new Set(RELATIONS));
  const [hover, setHover] = useState<Hover | null>(null);
  const [input, setInput] = useState("");
  const searchRef = useRef<HTMLInputElement | null>(null);
  const panelRef = useRef<HTMLElement | null>(null);
  const headingRef = useRef<HTMLHeadingElement | null>(null);

  const adjacency = useMemo(() => buildAdjacency(edges), [edges]);
  const nodeById = useMemo(() => new Map(nodes.map((n) => [n.id, n])), [nodes]);
  const relationCounts = useMemo(() => {
    const counts = new Map<GraphRelation, number>();
    for (const e of edges) counts.set(e.relation, (counts.get(e.relation) ?? 0) + 1);
    return counts;
  }, [edges]);
  const conceptCount = useMemo(() => nodes.filter((n) => n.data.kind === "concept").length, [nodes]);

  const focusNode = focusId ? nodeById.get(focusId) : undefined;
  const matchCount = input && input !== focusNode?.label ? matchNodes(nodes, input).length : null;
  const outCount = focusNode ? visibleLinks(adjacency, focusNode.id, "out", mode, relations).length : 0;
  const inCount = focusNode ? visibleLinks(adjacency, focusNode.id, "in", mode, relations).length : 0;
  const mutualCount = focusNode ? visibleLinks(adjacency, focusNode.id, "mutual", mode, relations).length : 0;

  const focus = (id: string | null) => {
    setFocusId(id);
    setHover(null);
  };

  useEffect(() => {
    const url = new URL(window.location.href);
    if (focusId) url.searchParams.set("node", focusId);
    else url.searchParams.delete("node");
    window.history.replaceState(null, "", url);
  }, [focusId]);

  const onEscape = useEffectEvent((e: KeyboardEvent) => {
    if (e.key !== "Escape" || e.defaultPrevented || !focusId) return;
    if (panelRef.current?.contains(document.activeElement)) searchRef.current?.focus();
    setFocusId(null);
    setHover(null);
  });
  useEffect(() => {
    const handler = (e: KeyboardEvent) => onEscape(e);
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, []);

  let placeholder: string | null = null;
  if (nodes.length === 0) {
    if (graphError) placeholder = `The graph couldn't load: ${graphError}`;
    else if (graphLoading || backendStatus !== "online") placeholder = "Loading the graph…";
    else placeholder = "No graph yet. Harvest a corpus to build one.";
  }

  return (
    <>
      <div className="flex flex-col overflow-hidden rounded-xl bg-white ring-line">
        <div className="flex flex-wrap items-end gap-x-6 gap-y-3 border-b border-line p-4">
          <div className="flex min-w-[min(260px,100%)] flex-1 flex-col gap-1">
            <Autocomplete
              size="small"
              options={nodes}
              value={focusNode ?? null}
              inputValue={input}
              onInputChange={(_, v) => setInput(v)}
              onChange={(_, n) => focus(n?.id ?? null)}
              filterOptions={(_, state) => matchNodes(nodes, state.inputValue)}
              getOptionKey={(n) => n.id}
              getOptionLabel={(n) => n.label}
              isOptionEqualToValue={(a, b) => a.id === b.id}
              renderOption={({ key, ...props }, n) => (
                <li key={key} {...props} className={`${props.className ?? ""} flex justify-between gap-3`}>
                  <span className="truncate text-sm">{n.label}</span>
                  <span className="shrink-0 font-mono text-[12px] text-ink-3">
                    {n.data.kind === "concept" ? "CONCEPT" : n.data.year}
                  </span>
                </li>
              )}
              renderInput={(p) => <TextField {...p} inputRef={searchRef} label="Find a node" />}
              slotProps={{ paper: { className: "ring-line" } }}
            />
            <span role="status" className="sr-only">
              {matchCount === null
                ? ""
                : `${matchCount}${matchCount === MATCH_CAP ? " or more" : ""} matches`}
            </span>
          </div>

          <div className="flex flex-col gap-1">
            <span id="graph-direction-label" className="font-mono text-[12px] uppercase tracking-wider text-ink-3">
              Direction
            </span>
            <ToggleButtonGroup
              size="small"
              exclusive
              value={mode}
              disabled={!focusNode}
              aria-labelledby="graph-direction-label"
              aria-describedby={focusNode ? undefined : "graph-direction-hint"}
              onChange={(_, v: Direction | null) => v && setMode(v)}
            >
              <ToggleButton value="all">All</ToggleButton>
              <ToggleButton value="out">Out</ToggleButton>
              <ToggleButton value="in">In</ToggleButton>
              <ToggleButton value="mutual">Mutual</ToggleButton>
            </ToggleButtonGroup>
            {focusNode ? null : (
              <span id="graph-direction-hint" className="text-[12px] text-ink-3">
                Select a node to filter by direction
              </span>
            )}
          </div>

          <div className="flex flex-col gap-1">
            <span id="graph-relations-label" className="font-mono text-[12px] uppercase tracking-wider text-ink-3">
              Relations
            </span>
            <ToggleButtonGroup
              size="small"
              value={RELATIONS.filter((r) => relations.has(r))}
              aria-labelledby="graph-relations-label"
              onChange={(_, v: GraphRelation[]) => setRelations(new Set(v))}
            >
              {RELATIONS.map((r) => (
                <ToggleButton key={r} value={r}>
                  {RELATION_TOGGLE[r]}
                </ToggleButton>
              ))}
            </ToggleButtonGroup>
          </div>
        </div>

        <div className="grid grid-cols-1 lg:grid-cols-[minmax(0,1fr)_1px_340px]">
          <figure className="m-0 flex min-w-0 flex-col bg-bg-faint">
            <div className="h-[60vh] min-h-[360px] lg:h-[600px]">
              {placeholder ? (
                <p className="m-0 flex h-full items-center justify-center p-8 text-center text-sm text-ink-2">
                  {placeholder}
                </p>
              ) : (
                <GraphCanvas
                  nodes={nodes}
                  edges={edges}
                  adjacency={adjacency}
                  focusId={focusId}
                  hover={hover}
                  mode={mode}
                  relations={relations}
                  captionId={SUMMARY_ID}
                  onFocusChange={focus}
                />
              )}
            </div>
            <figcaption
              className="flex flex-col gap-2 border-t border-line px-4 py-3 text-[12px] text-ink-2"
            >
              <Legend />
              <span id={SUMMARY_ID}>
                Relationship graph of {(nodes.length - conceptCount).toLocaleString()} papers and{" "}
                {conceptCount.toLocaleString()} concepts:{" "}
                {RELATIONS.map((r) => `${(relationCounts.get(r) ?? 0).toLocaleString()} ${RELATION_LABEL[r].toLowerCase()}`).join(", ")}{" "}
                edges.{" "}
                <a
                  href={`${API_BASE}/api/v1/graph/snapshot`}
                  className="text-accent underline underline-offset-2"
                >
                  Graph data (JSON)
                </a>
              </span>
            </figcaption>
          </figure>
          <div className="hidden bg-line lg:block" />
          <section
            ref={panelRef}
            aria-label="Focused node"
            className="max-h-[720px] min-h-0 overflow-y-auto border-t border-line lg:border-t-0"
          >
            <NodePanel
              focusId={focusId}
              node={focusNode}
              nodeById={nodeById}
              adjacency={adjacency}
              mode={mode}
              relations={relations}
              headingRef={headingRef}
              onFocus={focus}
              onHover={setHover}
            />
          </section>
        </div>
      </div>
      <p role="status" className="sr-only">
        {focusNode ? `${focusNode.label}: ${outCount} out, ${inCount} in, ${mutualCount} mutual` : ""}
      </p>
    </>
  );
}

function Legend() {
  const item = (label: string, stroke: string, arrow: "away" | "toward" | null) => (
    <span className="inline-flex items-center gap-1.5">
      <svg width="34" height="10" viewBox="0 0 34 10" aria-hidden="true" className="shrink-0">
        <circle cx="4" cy="5" r="3" fill={color.slab} />
        <line x1="8" y1="5" x2={arrow === "away" ? 26 : 33} y2="5" stroke={stroke} strokeWidth="2" />
        {arrow === "away" ? <path d="M26 1 L33 5 L26 9 Z" fill={stroke} /> : null}
        {arrow === "toward" ? <path d="M16 1 L9 5 L16 9 Z" fill={stroke} /> : null}
      </svg>
      {label}
    </span>
  );
  return (
    <span className="flex flex-wrap gap-x-4 gap-y-1">
      {item("Out — this node points to it", color.ink, "away")}
      {item("In — it points to this node", color.accent, "toward")}
      {item("Mutual — similar or co-authored, no arrow", color.ink3, null)}
      <span className="font-mono uppercase">Mono caps = concept</span>
    </span>
  );
}
