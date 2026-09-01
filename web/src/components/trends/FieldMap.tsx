"use client";
import Typography from "@mui/material/Typography";
import { useRouter } from "next/navigation";
import { useMemo, useState } from "react";

import { useAppStore } from "@/store";

import { buildFieldMap, MIN_COMMUNITY_SIZE } from "./field-map-layout";

const GROUND = "#959A9D";
const SLAB = "#0F0F10";
const EDGE = "#D8DBDC";

/**
 * The coarse-grained field map — paper Fig. 2c. One node per community above
 * the size threshold, edges for inter-community backbone links above a count
 * threshold, the ten largest labelled.
 *
 * Deliberately not the paper-level graph: with 67 communities colour cannot
 * encode identity, but at this granularity a label can, which is what makes the
 * map legible. Selecting a community opens it in the reader.
 */
export function FieldMap() {
  const communities = useAppStore((s) => s.communities);
  const links = useAppStore((s) => s.communityLinks);
  const selectCommunity = useAppStore((s) => s.selectCommunity);
  const router = useRouter();
  const [hover, setHover] = useState<number | null>(null);

  const map = useMemo(() => buildFieldMap(communities, links), [communities, links]);

  const box = useMemo(() => {
    if (map.nodes.length === 0) return { x: 0, y: 0, w: 100, h: 100 };
    const pad = 90;
    const xs = map.nodes.map((n) => n.x);
    const ys = map.nodes.map((n) => n.y);
    const x = Math.min(...xs) - pad;
    const y = Math.min(...ys) - pad;
    return { x, y, w: Math.max(...xs) + pad - x, h: Math.max(...ys) + pad - y };
  }, [map]);

  if (map.nodes.length === 0) {
    return (
      <Typography variant="caption" className="text-ink-2">
        No communities of {MIN_COMMUNITY_SIZE}+ papers yet — harvest a larger corpus.
      </Typography>
    );
  }

  const open = (slug: string) => {
    selectCommunity(slug);
    router.push(`/reader?c=${encodeURIComponent(slug)}`);
  };

  return (
    <figure className="m-0 flex flex-col gap-3">
      <svg
        viewBox={`${box.x} ${box.y} ${box.w} ${box.h}`}
        className="h-auto w-full rounded-xl bg-white ring-line"
        role="img"
        aria-labelledby="fieldmap-caption"
      >
        <g>
          {map.edges.map((e, i) => (
            <line
              key={i}
              x1={e.source.x}
              y1={e.source.y}
              x2={e.target.x}
              y2={e.target.y}
              stroke={
                hover !== null && (e.source.id === hover || e.target.id === hover) ? SLAB : EDGE
              }
              strokeWidth={hover !== null && (e.source.id === hover || e.target.id === hover) ? 2 : 1}
            />
          ))}
        </g>
        {map.nodes.map((n) => (
          <g
            key={n.id}
            className="cursor-pointer"
            onMouseEnter={() => setHover(n.id)}
            onMouseLeave={() => setHover(null)}
            onClick={() => open(n.slug)}
          >
            <circle
              cx={n.x}
              cy={n.y}
              r={n.r}
              fill={hover === n.id ? SLAB : GROUND}
              stroke="#FFFFFF"
              strokeWidth={2}
            />
            {n.labelled || hover === n.id ? (
              <text
                x={n.x}
                y={n.y + n.r + 15}
                textAnchor="middle"
                fontSize={13}
                fill="#1D1F20"
                style={{ fontFamily: "var(--font-mono)" }}
              >
                {n.label}
              </text>
            ) : null}
          </g>
        ))}
      </svg>

      <figcaption
        id="fieldmap-caption"
        className="m-0 font-mono text-[11px] leading-4 text-ink-3"
      >
        {map.nodes.length} communities of {MIN_COMMUNITY_SIZE}+ papers ·{" "}
        {map.edges.length} inter-community links above {map.threshold} shared edges · circle area
        is community size · the ten largest are labelled. Select a community to open it in the
        reader.
      </figcaption>

      <ul className="sr-only">
        {map.nodes.map((n) => (
          <li key={n.id}>
            {n.label}: {n.size.toLocaleString()} papers
          </li>
        ))}
      </ul>
    </figure>
  );
}
