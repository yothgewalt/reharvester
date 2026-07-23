"use client";
import LinearProgress from "@mui/material/LinearProgress";
import Typography from "@mui/material/Typography";


export function BootSplash() {
  return (
    <div className="fixed inset-0 flex flex-col items-center justify-center gap-4 bg-white">
      <Typography variant="overline" className="text-accent">
        Reharvester
      </Typography>
      <LinearProgress className="w-64" />
      <Typography variant="caption" className="text-ink-2">
        Verifying local environment…
      </Typography>
    </div>
  );
}
