import type { TrendPoint } from "@/types/domain";

/**
 * Year-by-year prevalence for one phrase, as an inline SVG polyline.
 *
 * The shape is the point: a term that peaked mid-corpus and fell away and one
 * still rising can share a lift value, and only the trajectory separates them.
 */
export function Sparkline({ points, label }: { points: TrendPoint[]; label: string }) {
  if (points.length < 2) return null;

  const w = 132;
  const h = 28;
  const peak = Math.max(...points.map((p) => p.prevalence));
  if (peak <= 0) return null;

  const x = (i: number) => (i / (points.length - 1)) * (w - 2) + 1;
  const y = (v: number) => h - 1 - (v / peak) * (h - 2);
  const line = points.map((p, i) => `${x(i).toFixed(1)},${y(p.prevalence).toFixed(1)}`).join(" ");
  const area = `1,${h - 1} ${line} ${(w - 1).toFixed(1)},${h - 1}`;
  const last = points[points.length - 1];

  return (
    <svg
      viewBox={`0 0 ${w} ${h}`}
      width={w}
      height={h}
      role="img"
      aria-label={`${label}: peak ${(peak * 100).toFixed(1)}% of abstracts in ${
        points.reduce((a, b) => (b.prevalence > a.prevalence ? b : a)).year
      }, ${(last.prevalence * 100).toFixed(1)}% in ${last.year}`}
      className="overflow-visible"
    >
      <polygon points={area} fill="currentColor" opacity={0.08} />
      <polyline
        points={line}
        fill="none"
        stroke="currentColor"
        strokeWidth={1.25}
        strokeLinejoin="round"
        strokeLinecap="round"
      />
      <circle cx={x(points.length - 1)} cy={y(last.prevalence)} r={1.75} fill="currentColor" />
    </svg>
  );
}
