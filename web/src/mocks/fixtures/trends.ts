import type { TrendKeyword } from "@/types/domain";

export const TREND_YEARS = { first: 2015, last: 2026 } as const;





const SERIES: Array<[string, number, number, number, number]> = [
  ["Graph Neural Networks", 14, 2, 2023, 46],
  ["Low-Rank Adaptation", 4, 1, 2021, 18],
  ["Sparse Autoencoders", 2, 0.5, 2023, 30],
  ["Retrieval-Augmented Generation", 6, 1, 2022, 24],
  ["Agentic Workflows", 1, 0.5, 2024, 40],
  ["Mechanistic Interpretability", 3, 1, 2021, 14],
  ["Flow Matching", 2, 0.5, 2023, 16],
  ["Mixture of Experts", 8, 1.5, 2023, 12],
  ["Chain-of-Thought Prompting", 2, 0.5, 2022, 10],
  ["Federated Learning", 20, 4, 2019, 2],
  ["Diffusion Models", 12, 3, 2021, 8],
  ["Knowledge Graph Completion", 15, 2, 2017, 0],
  ["Prompt Tuning", 3, 1, 2021, 6],
  ["Model Merging", 1, 0.5, 2023, 9],
  ["Vector Databases", 2, 0.5, 2022, 8],
  ["Consistency Models", 1, 0.3, 2023, 7],
  ["Speculative Decoding", 1, 0.3, 2023, 11],
  ["Constitutional AI", 1, 0.2, 2022, 5],
  ["Multi-Agent Coordination", 4, 0.8, 2023, 13],
  ["Neural Theorem Proving", 2, 0.4, 2020, 3],
  ["Quantization-Aware Training", 5, 1, 2022, 6],
  ["Video Diffusion", 1, 0.3, 2023, 12],
  ["Process Supervision", 1, 0.2, 2023, 8],
  ["Differential Privacy", 10, 2, 2018, 1],
  ["Split Learning", 3, 0.6, 2019, 1],
];

export interface TrendRow {
  keyword: string;
  year: number;
  count: number;
}

export const TREND_ROWS: TrendRow[] = SERIES.flatMap(([keyword, base, slope, takeoff, tSlope]) => {
  const rows: TrendRow[] = [];
  for (let year = TREND_YEARS.first; year <= TREND_YEARS.last; year++) {
    const count = Math.max(
      0,
      Math.round(base + slope * (year - TREND_YEARS.first) + (year >= takeoff ? tSlope * (year - takeoff) : 0)),
    );
    rows.push({ keyword, year, count });
  }
  return rows;
});


function regressionSlope(points: Array<[number, number]>): number {
  const n = points.length;
  if (n < 2) return 0;
  let sx = 0,
    sy = 0,
    sxy = 0,
    sxx = 0;
  for (const [x, y] of points) {
    sx += x;
    sy += y;
    sxy += x * y;
    sxx += x * x;
  }
  const denom = n * sxx - sx * sx;
  return denom === 0 ? 0 : (n * sxy - sx * sy) / denom;
}

export function computeTrends([start, end]: [number, number]): TrendKeyword[] {
  const byKeyword = new Map<string, Array<[number, number]>>();
  for (const row of TREND_ROWS) {
    if (row.year < start || row.year > end) continue;
    const arr = byKeyword.get(row.keyword) ?? [];
    arr.push([row.year, row.count]);
    byKeyword.set(row.keyword, arr);
  }
  return [...byKeyword.entries()]
    .map(([keyword, points]) => ({
      keyword,
      count: points.reduce((acc, [, c]) => acc + c, 0),
      growthRate: Math.round(regressionSlope(points) * 100) / 100,
    }))
    .sort((a, b) => b.growthRate - a.growthRate)
    .slice(0, 5)
    .map((t, i) => ({ ...t, rank: i + 1 }));
}
