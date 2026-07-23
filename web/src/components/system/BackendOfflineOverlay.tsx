"use client";
import Button from "@mui/material/Button";
import Modal from "@mui/material/Modal";
import Typography from "@mui/material/Typography";
import { useEffect, useRef, useState } from "react";

import { reportBackendFailure } from "@/lib/api/failure-bus";
import { useAppStore } from "@/store";

const UVICORN_CMD = "uvicorn app.main:app --reload --port 8000";
const MOCK_HINT = "# or demo without a backend: NEXT_PUBLIC_USE_MOCKS=true in .env.local";


export function BackendOfflineOverlay() {
  const backendStatus = useAppStore((s) => s.backendStatus);
  const [copied, setCopied] = useState(false);
  const copyTimer = useRef<number | null>(null);

  useEffect(
    () => () => {
      if (copyTimer.current !== null) window.clearTimeout(copyTimer.current);
    },
    [],
  );

  const handleCopy = () => {
    void navigator.clipboard.writeText(UVICORN_CMD).then(() => {
      setCopied(true);
      if (copyTimer.current !== null) window.clearTimeout(copyTimer.current);
      copyTimer.current = window.setTimeout(() => setCopied(false), 1500);
    });
  };

  return (
    <Modal
      open={backendStatus === "offline"}
      aria-labelledby="backend-offline-title"
      aria-describedby="backend-offline-desc"
    >
      <div className="absolute left-1/2 top-1/2 w-[560px] max-w-[90vw] -translate-x-1/2 -translate-y-1/2 rounded-xl bg-white p-8 outline-none ring-line flex flex-col gap-4">
        <Typography variant="overline" className="text-accent">
          LOCAL BACKEND
        </Typography>
        <Typography variant="h5" component="h2" id="backend-offline-title">
          Backend offline
        </Typography>
        <Typography variant="body2" className="text-ink-2" id="backend-offline-desc">
          The FastAPI server on port 8000 is unreachable. Inputs are blocked until it comes
          back. Health checks retry every 10 seconds.
        </Typography>
        <pre className="overflow-x-auto rounded-lg bg-slab p-4 font-mono text-[13px] text-ink-inverse">
          {UVICORN_CMD}
          {"\n"}
          {MOCK_HINT}
        </pre>
        <div>
          <Button variant="outlined" size="small" onClick={handleCopy}>
            {copied ? "Copied" : "Copy command"}
          </Button>
        </div>
        <div className="flex items-center gap-3">
          <Button variant="contained" autoFocus onClick={() => reportBackendFailure()}>
            Retry now
          </Button>
        </div>
      </div>
    </Modal>
  );
}
