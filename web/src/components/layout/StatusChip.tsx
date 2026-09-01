"use client";
import Chip from "@mui/material/Chip";

import { useAppStore } from "@/store";
import type { BackendStatus } from "@/store/system-slice";

const statusMap: Record<BackendStatus, { dotClass: string; label: string }> = {
  online: { dotClass: "bg-ok", label: "Local Status: Ready" },
  offline: { dotClass: "bg-error", label: "Local Status: Offline" },
  checking: { dotClass: "bg-ink-4", label: "Local Status: Checking…" },
};


const tierAdds: Record<string, string> = {
  T0: "inverted index only",
  T1: "with TF-IDF",
  T2: "with the k-NN backbone",
  T3: "with the neural encoder",
};

export function StatusChip() {
  const backendStatus = useAppStore((s) => s.backendStatus);
  const tier = useAppStore((s) => s.capabilityTier);
  const { dotClass, label } = statusMap[backendStatus];
  const showTier = tier !== null && backendStatus === "online";

  return (
    <span aria-live="polite" className="flex items-center gap-2">
      <Chip
        variant="outlined"
        icon={<span className={`h-2 w-2 rounded-full ${dotClass}`} />}
        label={label}
      />
      {showTier ? (
        <Chip
          variant="outlined"
          label={`Tier ${tier}`}
          title={`Capability ladder: ${tier} — ${tierAdds[tier] ?? ""}`}
        />
      ) : null}
    </span>
  );
}
