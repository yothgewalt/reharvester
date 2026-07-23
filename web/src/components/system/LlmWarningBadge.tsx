"use client";
import Chip from "@mui/material/Chip";
import Tooltip from "@mui/material/Tooltip";

import { useAppStore } from "@/store";

const TOOLTIP =
  "Semantic syntheses are unavailable — the system has fallen back to standard query search. Start Ollama or llama.cpp to restore Graph-RAG.";


export function LlmWarningBadge() {
  const llmStatus = useAppStore((s) => s.llmStatus);

  return (
    <span aria-live="polite">
      {llmStatus === "unreachable" ? (
        <Tooltip title={TOOLTIP}>
          <Chip
            icon={<span className="h-2 w-2 rounded-full bg-error" />}
            label="Local LLM unreachable"
            sx={{
              backgroundColor: "var(--color-warn-bg)",
              color: "var(--color-warn)",
            }}
          />
        </Tooltip>
      ) : null}
    </span>
  );
}
