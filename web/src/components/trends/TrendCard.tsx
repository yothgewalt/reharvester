import Card from "@mui/material/Card";
import Typography from "@mui/material/Typography";

import type { TrendKeyword } from "@/types/domain";

import { Sparkline } from "./Sparkline";

export interface TrendCardProps {
  trend: TrendKeyword;
  index: number;
  animate: boolean;
}


export function TrendCard({ trend, index, animate }: TrendCardProps) {
  const rank = String(trend.rank).padStart(2, "0");
  const negative = trend.growthRate < 0;
  const slope = `${negative ? "▼" : "▲"} ${negative ? "" : "+"}${trend.growthRate.toFixed(2)}`;

  return (
    <Card
      className={animate ? "card-in" : undefined}
      style={animate ? { animationDelay: `${index * 30}ms` } : undefined}
    >
      <div className="flex flex-col gap-2 p-5">
        <span className="font-mono text-[13px] text-ink-3">{rank}</span>
        <Typography variant="h6" component="h3" className="line-clamp-2">
          {trend.keyword}
        </Typography>
        <div className="font-mono text-sm text-ink">
          <span title="Smoothed log2 prevalence lift, late window against the preceding one">
            {slope} log₂
          </span>
          <span className="text-ink-2"> · {trend.count} works</span>
        </div>
        {trend.trajectory ? (
          <div className="text-ink-3">
            <Sparkline points={trend.trajectory} label={trend.keyword} />
            <div className="mt-1 flex justify-between font-mono text-[11px] text-ink-3">
              <span>{trend.trajectory[0]?.year}</span>
              <span>{trend.trajectory[trend.trajectory.length - 1]?.year}</span>
            </div>
          </div>
        ) : null}
      </div>
    </Card>
  );
}
