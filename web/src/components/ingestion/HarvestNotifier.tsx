"use client";

import Alert from "@mui/material/Alert";
import AlertTitle from "@mui/material/AlertTitle";
import Snackbar from "@mui/material/Snackbar";
import Link from "next/link";

import { useAppStore } from "@/store";

const SUCCESS_HIDE_MS = 10_000;

/** App-wide toast for the outcome of the last harvest, so it lands on whatever page is open. */
export function HarvestNotifier() {
  const notice = useAppStore((s) => s.harvestNotice);
  const dismiss = useAppStore((s) => s.dismissHarvestNotice);

  return (
    <Snackbar
      open={notice !== null}
      autoHideDuration={notice?.ok ? SUCCESS_HIDE_MS : null}
      onClose={(_, reason) => {
        if (reason !== "clickaway") dismiss();
      }}
      anchorOrigin={{ vertical: "bottom", horizontal: "center" }}
    >
      {notice ? (
        <Alert
          severity={notice.ok ? "success" : "error"}
          variant="outlined"
          role={notice.ok ? "status" : "alert"}
          onClose={dismiss}
          className="max-w-[560px] bg-white"
          sx={{ boxShadow: "none" }}
        >
          <AlertTitle>{notice.ok ? "Harvest complete" : "Harvest failed"}</AlertTitle>
          <span className="flex flex-wrap items-baseline gap-x-3 gap-y-1">
            <span>
              {notice.name}
              {notice.ok
                ? ` — ${notice.docsIngested.toLocaleString()} papers ingested.`
                : " — see the crawl log for what went wrong."}
            </span>
            <Link
              href={`/projects/detail?id=${encodeURIComponent(notice.projectId)}`}
              onClick={dismiss}
              className="text-accent underline underline-offset-2"
            >
              Open project
            </Link>
          </span>
        </Alert>
      ) : undefined}
    </Snackbar>
  );
}
