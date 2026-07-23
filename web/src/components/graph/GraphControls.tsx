"use client";

import Chip from "@mui/material/Chip";
import FormControlLabel from "@mui/material/FormControlLabel";
import Switch from "@mui/material/Switch";
import ToggleButton from "@mui/material/ToggleButton";
import ToggleButtonGroup from "@mui/material/ToggleButtonGroup";
import Tooltip from "@mui/material/Tooltip";

import { useAppStore } from "@/store";
import type { GraphKind, LayoutPreset } from "@/types/domain";

const KIND_OPTIONS: ReadonlyArray<{ value: GraphKind; label: string }> = [
  { value: "knowledge", label: "Knowledge" },
  { value: "cooccurrence", label: "Co-occurrence" },
];

const LAYOUT_OPTIONS: ReadonlyArray<{ value: LayoutPreset; label: string }> = [
  { value: "forceDirected2d", label: "Force 2D" },
  { value: "forceDirected3d", label: "Force 3D" },
  { value: "concentric2d", label: "Concentric 2D" },
  { value: "concentric3d", label: "Concentric 3D" },
  { value: "circular2d", label: "Circular" },
];

export function GraphControls() {
  const graphKind = useAppStore((s) => s.graphKind);
  const setGraphKind = useAppStore((s) => s.setGraphKind);
  const activeLayout = useAppStore((s) => s.activeLayout);
  const setLayout = useAppStore((s) => s.setLayout);
  const isGapAnalysisOverlayActive = useAppStore((s) => s.isGapAnalysisOverlayActive);
  const toggleGapOverlay = useAppStore((s) => s.toggleGapOverlay);
  const nodes = useAppStore((s) => s.nodes);
  const edges = useAppStore((s) => s.edges);

  const gapsDisabled = graphKind !== "knowledge";

  return (
    <div className="flex flex-wrap items-center gap-4">
      <ToggleButtonGroup
        exclusive
        size="small"
        value={graphKind}
        onChange={(_, value: GraphKind | null) => {
          if (value) setGraphKind(value);
        }}
        aria-label="Graph type"
      >
        {KIND_OPTIONS.map((opt) => (
          <ToggleButton key={opt.value} value={opt.value}>
            {opt.label}
          </ToggleButton>
        ))}
      </ToggleButtonGroup>
      <span className="h-6 w-px bg-line" aria-hidden />
      <ToggleButtonGroup
        exclusive
        size="small"
        value={activeLayout}
        onChange={(_, value: LayoutPreset | null) => {
          if (value) setLayout(value);
        }}
        aria-label="Graph layout"
      >
        {LAYOUT_OPTIONS.map((opt) => (
          <ToggleButton key={opt.value} value={opt.value}>
            {opt.label}
          </ToggleButton>
        ))}
      </ToggleButtonGroup>
      <Tooltip title={gapsDisabled ? "Knowledge graph only" : ""}>
        <FormControlLabel
          control={
            <Switch
              checked={isGapAnalysisOverlayActive}
              onChange={() => void toggleGapOverlay()}
              disabled={gapsDisabled}
            />
          }
          label="Gaps layer"
          slotProps={{ typography: { variant: "caption" } }}
        />
      </Tooltip>
      <div className="flex-1" />
      <Chip
        variant="outlined"
        className="font-mono"
        label={`${nodes.length.toLocaleString()} nodes · ${edges.length.toLocaleString()} edges`}
      />
    </div>
  );
}

export default GraphControls;
