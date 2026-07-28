"use client";

import Alert from "@mui/material/Alert";
import Button from "@mui/material/Button";
import dynamic from "next/dynamic";
import { useEffect, useState } from "react";

import { SectionShell } from "@/components/layout/SectionShell";
import { usePrefersReducedMotion } from "@/components/layout/usePrefersReducedMotion";
import { useEnsureGraphSnapshot } from "@/lib/useEnsureGraphSnapshot";
import { isWebGLSupported } from "@/lib/webgl";
import { useAppStore } from "@/store";

import { GraphControls } from "./GraphControls";
import { GraphSkeleton } from "./GraphSkeleton";


const DynGraphView = dynamic(() => import("./graph-view"), {
  ssr: false,
  loading: () => <GraphSkeleton />,
});

type CanvasState = "pending" | "unsupported" | "ok" | "lost";


export function GraphExplorer() {
  const prefersReducedMotion = usePrefersReducedMotion();
  const setWebglSupported = useAppStore((s) => s.setWebglSupported);
  const loadGraphSnapshot = useAppStore((s) => s.loadGraphSnapshot);
  const graphError = useAppStore((s) => s.graphError);
  const graphKind = useAppStore((s) => s.graphKind);
  const selectedNodeId = useAppStore((s) => s.selectedNodeId);
  const nodeCount = useAppStore((s) => s.nodes.length);
  const [canvasState, setCanvasState] = useState<CanvasState>("pending");
  const [canvasKey, setCanvasKey] = useState(0);

  useEffect(() => {
    const ok = isWebGLSupported();
    setWebglSupported(ok);
    setCanvasState(ok ? "ok" : "unsupported");
  }, [setWebglSupported]);

  useEnsureGraphSnapshot();

  return (
    <div className="flex flex-col gap-4">
      <GraphControls />
      {graphKind === "knowledge" && !selectedNodeId ? (
        <span className="font-mono text-[13px] text-ink-3">
          Select a node to reveal its relations (node —[relation]→ node).
        </span>
      ) : null}
      <div className="overflow-hidden rounded-md bg-card-dark ring-line-dark">
        {canvasState === "pending" ? <GraphSkeleton /> : null}
        {canvasState === "unsupported" ? (
          <Alert severity="warning" className="m-6">
            [WebGL Engine Unreachable — Fallback Mode Active] This browser cannot
            render the graph canvas. The corpus and wiki reader below remain fully
            functional.
          </Alert>
        ) : null}
        {canvasState === "lost" ? (
          <Alert
            severity="warning"
            className="m-6"
            action={
              <Button
                variant="outlined"
                size="small"
                onClick={() => {
                  setCanvasKey((k) => k + 1);
                  setCanvasState("ok");
                }}
              >
                Retry
              </Button>
            }
          >
            The graph canvas lost its WebGL context. Retry to restore rendering.
          </Alert>
        ) : null}
        {canvasState === "ok" && graphError && nodeCount === 0 ? (
          <Alert
            severity="error"
            className="m-6"
            action={
              <Button variant="outlined" size="small" onClick={() => void loadGraphSnapshot()}>
                Retry
              </Button>
            }
          >
            Could not load the graph snapshot: {graphError}
          </Alert>
        ) : null}
        {canvasState === "ok" ? (
          <DynGraphView
            key={canvasKey}
            reducedMotion={prefersReducedMotion}
            onContextLost={() => setCanvasState("lost")}
          />
        ) : null}
      </div>
    </div>
  );
}

export function GraphSection() {
  const graphKind = useAppStore((s) => s.graphKind);

  return (
    <SectionShell
      id="graph"
      eyebrow="Graph explorer"
      title="Interactive canvas"
      description={
        graphKind === "cooccurrence"
          ? "Co-occurrence graph — concepts weighted by shared appearances across harvested works."
          : "Knowledge graph — papers and concepts linked by citation paths, sized by PageRank."
      }
    >
      <GraphExplorer />
    </SectionShell>
  );
}

export default GraphSection;
