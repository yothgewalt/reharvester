"use client";
import TextField from "@mui/material/TextField";
import Typography from "@mui/material/Typography";
import { useMemo, useRef, useState } from "react";

import { useAppStore } from "@/store";
import type { Community } from "@/types/domain";

const PAPER_CAP = 200;

/**
 * Renders the community list, or paper search results when the query is
 * non-empty. Both modes are one listbox with a single tab stop; arrows move.
 */
export function CommunityRail() {
  const communities = useAppStore((s) => s.communities);
  const selectedSlug = useAppStore((s) => s.selectedCommunitySlug);
  const selectCommunity = useAppStore((s) => s.selectCommunity);
  const nodes = useAppStore((s) => s.nodes);
  const selectedDocId = useAppStore((s) => s.selectedDocId);
  const selectNode = useAppStore((s) => s.selectNode);

  const [query, setQuery] = useState("");
  const listRef = useRef<HTMLUListElement | null>(null);

  const papers = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return [];
    return nodes
      .filter((n) => n.data.kind === "paper" && n.label.toLowerCase().includes(q))
      .slice(0, PAPER_CAP);
  }, [nodes, query]);

  const searching = query.trim().length > 0;
  const count = searching ? papers.length : communities.length;

  const onKeyDown = (e: React.KeyboardEvent<HTMLUListElement>) => {
    const items = Array.from(
      listRef.current?.querySelectorAll<HTMLElement>('[role="option"]') ?? [],
    );
    if (items.length === 0) return;
    const active = items.findIndex((el) => el === document.activeElement);
    let next = -1;
    if (e.key === "ArrowDown") next = Math.min(items.length - 1, active + 1);
    else if (e.key === "ArrowUp") next = Math.max(0, active - 1);
    else if (e.key === "Home") next = 0;
    else if (e.key === "End") next = items.length - 1;
    if (next < 0) return;
    e.preventDefault();
    items[next].focus();
  };

  return (
    <div className="flex min-h-0 flex-col gap-3">
      <TextField
        size="small"
        label="Search papers"
        value={query}
        onChange={(e) => setQuery(e.target.value)}
        fullWidth
      />
      <Typography variant="overline" className="text-ink-3" aria-live="polite">
        {count} {searching ? "papers" : "communities"}
        {searching && papers.length === PAPER_CAP ? " (refine to narrow)" : ""}
      </Typography>

      <ul
        ref={listRef}
        role="listbox"
        aria-label={searching ? "Search results" : "Communities"}
        onKeyDown={onKeyDown}
        className="m-0 flex list-none flex-col gap-1 overflow-y-auto p-0"
      >
        {searching
          ? papers.map((n, i) => (
              <Row
                key={n.id}
                label={n.label}
                meta={String(n.data.year)}
                selected={selectedDocId === n.data.docId}
                tabbable={i === 0}
                onSelect={() => selectNode(n.id, n.data.docId)}
              />
            ))
          : communities.map((c, i) => (
              <Row
                key={c.id}
                label={displayLabel(c)}
                title={c.label}
                ariaLabel={`${c.label}, ${c.size} papers`}
                meta={c.size.toLocaleString()}
                selected={selectedSlug === c.slug}
                tabbable={i === 0}
                onSelect={() => selectCommunity(c.slug)}
              />
            ))}
      </ul>
    </div>
  );
}

function Row({
  label,
  meta,
  title,
  ariaLabel,
  selected,
  tabbable,
  onSelect,
}: {
  label: string;
  meta: string;
  title?: string;
  ariaLabel?: string;
  selected: boolean;
  tabbable: boolean;
  onSelect(): void;
}) {
  return (
    <li className="contents">
      <button
        type="button"
        role="option"
        aria-selected={selected}
        aria-label={ariaLabel}
        title={title}
        tabIndex={tabbable || selected ? 0 : -1}
        onClick={onSelect}
        onKeyDown={(e) => {
          if (e.key === "Enter" || e.key === " ") {
            e.preventDefault();
            onSelect();
          }
        }}
        className={`flex w-full items-center justify-between gap-2 rounded-md px-3 py-2 text-left text-[13px] transition-colors ${
          selected ? "bg-slab text-ink-inverse" : "text-ink-1 hover:bg-bg-subtle"
        }`}
      >
        <span className="truncate">{label}</span>
        <span
          className={`shrink-0 font-mono text-[11px] ${selected ? "text-ink-inverse" : "text-ink-3"}`}
        >
          {meta}
        </span>
      </button>
    </li>
  );
}

/**
 * "reasoning / models" — the first term plus the first that is not a near
 * duplicate of it, since Louvain labels stack morphological variants
 * ("recommendation, recommender, user" -> "recommendation / user").
 */
export function displayLabel(c: Community): string {
  const [head, ...rest] = c.terms;
  if (!head) return c.label;
  const second = rest.find((t) => !related(head, t));
  return second ? `${head} / ${second}` : head;
}

function related(a: string, b: string): boolean {
  const x = a.toLowerCase();
  const y = b.toLowerCase();
  return x.includes(y) || y.includes(x) || x.slice(0, 6) === y.slice(0, 6);
}
