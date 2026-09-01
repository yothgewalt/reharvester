"use client";
import { ThemeProvider } from "@mui/material/styles";
import { type ReactNode, useEffect } from "react";

import { BootSplash } from "@/components/system/BootSplash";
import { useBackendHealth } from "@/lib/api/health";
import { useAppStore } from "@/store";
import { useProjectsStore } from "@/store/projects";
import { theme } from "@/theme/theme";

export function Providers({ children }: { children: ReactNode }) {
  return (
    <ThemeProvider theme={theme}>
      <HealthGate>{children}</HealthGate>
    </ThemeProvider>
  );
}

function HealthGate({ children }: { children: ReactNode }) {
  useBackendHealth();
  const backendStatus = useAppStore((s) => s.backendStatus);
  const loadJobs = useAppStore((s) => s.loadJobs);
  const hydrateProjects = useProjectsStore((s) => s.hydrate);

  // Projects and scheduled jobs live on the backend. Pull them once it answers,
  // so a reload does not present an empty scheduler and a stale project list.
  useEffect(() => {
    if (backendStatus !== "online") return;
    void hydrateProjects();
    void loadJobs();
  }, [backendStatus, hydrateProjects, loadJobs]);

  if (backendStatus === "checking") return <BootSplash />;

  return <>{children}</>;
}
