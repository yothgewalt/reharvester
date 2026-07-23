"use client";
import Slider from "@mui/material/Slider";
import Typography from "@mui/material/Typography";

import { useAppStore } from "@/store";
import { TREND_CURRENT_YEAR } from "@/store/trends-slice";

const MARKS = [
  { value: 3, label: "3 yr" },
  { value: 4, label: "4 yr" },
  { value: 5, label: "5 yr" },
];


export function TimeWindowSlider() {
  const selectedTimeWindow = useAppStore((s) => s.selectedTimeWindow);
  const setTimeWindow = useAppStore((s) => s.setTimeWindow);
  const recalculateTrends = useAppStore((s) => s.recalculateTrends);

  const [start, end] = selectedTimeWindow;
  const years = end - start + 1;

  return (
    <div className="flex w-64 flex-col gap-1">
      <Typography variant="caption" className="text-ink-2">
        Analysis window
      </Typography>
      <div className="px-2">
        <Slider
          value={years}
          min={3}
          max={5}
          step={1}
          marks={MARKS}
          aria-label="Analysis time window"
          getAriaValueText={(v) => `${v} years`}
          valueLabelDisplay="auto"
          onChange={(_event, value) => {
            const v = Array.isArray(value) ? value[0] : value;
            setTimeWindow([TREND_CURRENT_YEAR - (v - 1), TREND_CURRENT_YEAR]);
          }}
          onChangeCommitted={() => {
            void recalculateTrends();
          }}
        />
      </div>
    </div>
  );
}
