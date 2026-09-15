"use client";
import Button from "@mui/material/Button";
import Typography from "@mui/material/Typography";
import Link from "next/link";
import { type ReactNode, type RefObject, useState } from "react";

import { moveOptionFocus, Row } from "@/components/reader/CommunityRail";
import type { GraphNode, GraphRelation } from "@/types/domain";

import type { Hover } from "./GraphCanvas";
import {
  type Adjacency,
  type Direction,
  type Link as GraphLink,
  RELATION_LABEL,
  RELATIONS,
  visibleLinks,
} from "./graph-model";

const ROWS_PER_RELATION = 8;

const DIRECTION_HEADING = {
  out: { title: "Out", hint: "this node points to them" },
  in: { title: "In", hint: "they point to this node" },
} as const;

export interface NodePanelProps {
  focusId: string | null;
  node: GraphNode | undefined;
  nodeById: ReadonlyMap<string, GraphNode>;
  adjacency: ReadonlyMap<string, Adjacency>;
  mode: Direction;
  relations: ReadonlySet<GraphRelation>;
  /** attached to the panel heading; it receives keyboard focus after walking to a neighbour */
  headingRef: RefObject<HTMLHeadingElement | null>;
  onFocus(id: string | null): void;
  onHover(hover: Hover | null): void;
}

export function NodePanel({
  focusId,
  node,
  nodeById,
  adjacency,
  mode,
  relations,
  headingRef,
  onFocus,
  onHover,
}: NodePanelProps) {
  if (!focusId) {
    return (
      <div className="flex flex-col gap-2 p-5">
        <Typography variant="overline" component="h3" className="text-ink-3">
          No node focused
        </Typography>
        <p className="m-0 text-sm text-ink-2">
          Search for a paper or concept, or click a node in the graph, to list what it points to and
          what points to it.
        </p>
      </div>
    );
  }

  if (!node) {
    return (
      <div className="flex flex-col items-start gap-3 p-5">
        <Typography variant="h6" component="h3" ref={headingRef} tabIndex={-1}>
          Not in the current graph
        </Typography>
        <p className="m-0 text-sm text-ink-2">
          <code className="font-mono text-[13px]">{focusId}</code> isn&apos;t part of the loaded
          snapshot.
        </p>
        <Button variant="outlined" size="small" onClick={() => onFocus(null)}>
          Clear focus
        </Button>
      </div>
    );
  }

  const isPaper = node.data.kind === "paper";
  return (
    <div className="flex flex-col gap-5 p-5">
      <div className="flex flex-col gap-1">
        <Typography variant="overline" className="text-accent">
          {isPaper ? "Paper" : "Concept"}
        </Typography>
        <Typography
          variant="h6"
          component="h3"
          ref={headingRef}
          tabIndex={-1}
          className="break-words"
        >
          {node.label}
        </Typography>
        <p className="m-0 font-mono text-[12px] text-ink-3">
          {isPaper ? `${node.data.year} · ` : ""}
          {node.data.cluster}
        </p>
        {isPaper ? (
          <Link
            href={`/reader?doc=${encodeURIComponent(node.data.docId)}`}
            className="text-[13px] text-accent underline-offset-2 hover:underline"
          >
            Open in Reader
          </Link>
        ) : null}
      </div>

      {(["out", "in"] as const).map((dir) =>
        mode === "all" || mode === dir ? (
          <DirectionList
            key={`${focusId}:${dir}`}
            dir={dir}
            links={visibleLinks(adjacency, focusId, dir, mode, relations)}
            nodeById={nodeById}
            onFocus={onFocus}
            onHover={onHover}
            onWalk={() => headingRef.current?.focus()}
          />
        ) : null,
      )}
    </div>
  );
}

function DirectionList({
  dir,
  links,
  nodeById,
  onFocus,
  onHover,
  onWalk,
}: {
  dir: "out" | "in";
  links: GraphLink[];
  nodeById: ReadonlyMap<string, GraphNode>;
  onFocus(id: string): void;
  onHover(hover: Hover | null): void;
  onWalk(): void;
}) {
  const [expanded, setExpanded] = useState(false);
  const headingId = `graph-${dir}-heading`;
  const { title, hint } = DIRECTION_HEADING[dir];

  const groups = RELATIONS.map((relation) => ({
    relation,
    links: links.filter((l) => l.edge.relation === relation),
  })).filter((g) => g.links.length > 0);
  const truncated = groups.some((g) => g.links.length > ROWS_PER_RELATION);
  const firstEdgeId = groups[0]?.links[0]?.edge.id;

  return (
    <section aria-labelledby={headingId} className="flex flex-col gap-2">
      <Typography variant="overline" component="h4" id={headingId} className="text-ink">
        {title} · {links.length}
        <span className="normal-case tracking-normal text-ink-3"> — {hint}</span>
      </Typography>
      {links.length === 0 ? (
        <p className="m-0 text-[13px] text-ink-3">None under the current filters.</p>
      ) : (
        <ul
          role="listbox"
          aria-labelledby={headingId}
          onKeyDown={moveOptionFocus}
          className="m-0 flex list-none flex-col gap-1 p-0"
        >
          {groups.map((g) => (
            <GroupRows key={g.relation} relation={g.relation} total={g.links.length}>
              {(expanded ? g.links : g.links.slice(0, ROWS_PER_RELATION)).map((l) => {
                const other = nodeById.get(l.other);
                const label = other?.label ?? l.other;
                const weighted = g.relation === "similar-to" || g.relation === "nearest-to";
                return (
                  <Row
                    key={l.edge.id}
                    label={label}
                    title={label}
                    meta={weighted ? l.edge.weight.toFixed(2) : ""}
                    ariaLabel={`${label}, ${RELATION_LABEL[g.relation].toLowerCase()}${
                      weighted ? `, cosine ${l.edge.weight.toFixed(2)}` : ""
                    }`}
                    selected={false}
                    tabbable={l.edge.id === firstEdgeId}
                    onHighlight={(on) => onHover(on ? { node: l.other, edge: l.edge.id } : null)}
                    onSelect={() => {
                      onFocus(l.other);
                      onWalk();
                    }}
                  />
                );
              })}
            </GroupRows>
          ))}
        </ul>
      )}
      {truncated ? (
        <Button
          variant="text"
          size="small"
          className="self-start"
          aria-expanded={expanded}
          onClick={() => setExpanded((v) => !v)}
        >
          {expanded ? "Show fewer" : `Show all ${links.length}`}
        </Button>
      ) : null}
    </section>
  );
}

/** Visual relation header; hidden from the listbox's accessibility tree because
 *  each option's name already carries its relation. */
function GroupRows({
  relation,
  total,
  children,
}: {
  relation: GraphRelation;
  total: number;
  children: ReactNode;
}) {
  return (
    <>
      <li
        aria-hidden="true"
        className="px-3 pt-2 font-mono text-[12px] uppercase tracking-wider text-ink-3"
      >
        {RELATION_LABEL[relation]} · {total}
      </li>
      {children}
    </>
  );
}
