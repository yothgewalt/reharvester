"use client";

import Chip from "@mui/material/Chip";
import Typography from "@mui/material/Typography";

import { useAppStore } from "@/store";

export function ActiveCrawlers() {
  const schedulerProfiles = useAppStore((s) => s.schedulerProfiles);

  return (
    <div className="flex flex-col gap-2">
      <Typography variant="subtitle1">Active Crawlers</Typography>
      {schedulerProfiles.length === 0 ? (
        <Typography variant="caption" className="text-ink-2">
          No scheduled crawlers yet.
        </Typography>
      ) : (
        <div className="flex flex-col">
          {schedulerProfiles.map((profile) => (
            <div
              key={profile.id}
              className="flex items-center justify-between gap-4 border-t border-line py-2 first:border-t-0"
            >
              <div className="flex min-w-0 flex-col">
                <Typography variant="body2" className="font-medium">
                  {profile.name}
                </Typography>
                <Typography variant="caption" className="truncate text-ink-2">
                  {profile.keywords.join(", ")}
                </Typography>
              </div>
              <div className="flex shrink-0 items-center gap-3">
                <span className="font-mono text-[13px] text-ink-2">{profile.cron}</span>
                <Chip size="small" variant="outlined" label={profile.status} />
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
