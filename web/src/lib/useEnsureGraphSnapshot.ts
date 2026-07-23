"use client";
import { useEffect } from "react";

import { useAppStore } from "@/store";


export function useEnsureGraphSnapshot(): void {
  const nodeCount = useAppStore((s) => s.nodes.length);
  const backendStatus = useAppStore((s) => s.backendStatus);
  const loadGraphSnapshot = useAppStore((s) => s.loadGraphSnapshot);

  useEffect(() => {
    if (nodeCount > 0 || (backendStatus !== "online" && backendStatus !== "mocked")) return;
    const { graphLoading } = useAppStore.getState();
    if (!graphLoading) void loadGraphSnapshot();
  }, [nodeCount, backendStatus, loadGraphSnapshot]);
}
