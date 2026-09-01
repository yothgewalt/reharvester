"use client";
import { useEffect } from "react";

import { API_BASE } from "@/lib/config";
import { useAppStore } from "@/store";
import type { CapabilityTier } from "@/types/domain";

import { onBackendFailure } from "./failure-bus";

const OFFLINE_POLL_MS = 10_000;
const ONLINE_POLL_MS = 30_000;
const PING_TIMEOUT_MS = 3_000;


export function useBackendHealth(): void {
  useEffect(() => {
    let timer: ReturnType<typeof setTimeout> | null = null;
    let disposed = false;


    let pinging = false;

    const schedule = () => {
      if (disposed) return;
      const status = useAppStore.getState().backendStatus;
      const delay = status === "offline" ? OFFLINE_POLL_MS : ONLINE_POLL_MS;
      timer = setTimeout(ping, delay);
    };

    const ping = async () => {
      if (disposed || pinging) return;
      pinging = true;
      const { setBackendStatus, setLlmStatus, setCapabilityTier } = useAppStore.getState();
      try {
        const res = await fetch(`${API_BASE}/health`, {
          signal: AbortSignal.timeout(PING_TIMEOUT_MS),
        });
        if (!res.ok) throw new Error(`health ${res.status}`);
        const health = (await res.json()) as {
          llm?: "ok" | "unreachable";
          tier?: CapabilityTier;
        };
        setBackendStatus("online");
        setLlmStatus(health.llm ?? "unknown");
        setCapabilityTier(health.tier ?? null);
      } catch {
        setBackendStatus("offline");
      } finally {
        pinging = false;
        schedule();
      }
    };

    const offFailure = onBackendFailure(() => {
      if (timer) clearTimeout(timer);
      void ping();
    });

    void ping();

    return () => {
      disposed = true;
      if (timer) clearTimeout(timer);
      offFailure();
    };
  }, []);
}
