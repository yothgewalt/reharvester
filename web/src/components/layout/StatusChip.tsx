"use client";
import Chip from "@mui/material/Chip";

import { useAppStore } from "@/store";
import type { BackendStatus } from "@/store/system-slice";

const statusMap: Record<BackendStatus, { dotClass: string; label: string }> = {
  online: { dotClass: "bg-ok", label: "Local Status: Ready" },
  mocked: { dotClass: "bg-ok", label: "Local Status: Ready (mock)" },
  offline: { dotClass: "bg-error", label: "Local Status: Offline" },
  checking: { dotClass: "bg-ink-4", label: "Local Status: Checking…" },
};


export function StatusChip() {
  const backendStatus = useAppStore((s) => s.backendStatus);
  const { dotClass, label } = statusMap[backendStatus];

  return (
    <span aria-live="polite">
      <Chip
        variant="outlined"
        icon={<span className={`h-2 w-2 rounded-full ${dotClass}`} />}
        label={label}
      />
    </span>
  );
}
