"use client";
import { ThemeProvider } from "@mui/material/styles";
import type { ReactNode } from "react";

import { BackendOfflineOverlay } from "@/components/system/BackendOfflineOverlay";
import { BootSplash } from "@/components/system/BootSplash";
import { useBackendHealth } from "@/lib/api/health";
import { useAppStore } from "@/store";
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

  if (backendStatus === "checking") return <BootSplash />;

  return (
    <>
      {children}
      <BackendOfflineOverlay />
    </>
  );
}
