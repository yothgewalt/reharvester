"use client";

import FolderOutlined from "@mui/icons-material/FolderOutlined";
import ScheduleOutlined from "@mui/icons-material/ScheduleOutlined";
import TravelExploreOutlined from "@mui/icons-material/TravelExploreOutlined";
import Typography from "@mui/material/Typography";
import Link from "next/link";
import { usePathname } from "next/navigation";
import type { ComponentType, ReactNode } from "react";

import { AppHeader } from "./AppHeader";

interface NavItem {
  href: string;
  label: string;
  icon: ComponentType<{ fontSize?: "small"; className?: string }>;
}

const NAV_GROUPS: Array<{ label: string; items: NavItem[] }> = [
  {
    label: "Workspace",
    items: [
      { href: "/", label: "Harvest", icon: TravelExploreOutlined },
      { href: "/projects", label: "Projects", icon: FolderOutlined },
    ],
  },
  {
    label: "Automation",
    items: [{ href: "/scheduler", label: "Scheduler", icon: ScheduleOutlined }],
  },
];


export function AppShell({ children }: { children: ReactNode }) {
  const pathname = usePathname();

  return (
    <div className="flex h-screen overflow-hidden bg-white">
      <aside className="flex w-60 shrink-0 flex-col border-r border-line">
        <div className="flex h-14 shrink-0 items-center border-b border-line px-5">
          <Typography
            component="h1"
            className="font-display text-[16px] font-[430] leading-[1.3] tracking-[-0.03em]"
          >
            Reharvester
          </Typography>
        </div>
        <nav aria-label="Primary" className="flex flex-1 flex-col gap-6 overflow-y-auto p-3">
          {NAV_GROUPS.map((group) => (
            <div key={group.label} className="flex flex-col gap-1">
              <Typography variant="overline" className="px-2.5 text-ink-3">
                {group.label}
              </Typography>
              {group.items.map((item) => {

                const active =
                  pathname === item.href ||
                  (item.href !== "/" && pathname.startsWith(`${item.href}/`));
                const Icon = item.icon;
                return (
                  <Link
                    key={item.href}
                    href={item.href}
                    aria-current={active ? "page" : undefined}
                    className={`flex items-center gap-2.5 rounded-md px-2.5 py-2 text-sm transition-colors duration-150 ${
                      active
                        ? "bg-bg-subtle font-medium text-ink"
                        : "text-ink-2 hover:bg-bg-faint hover:text-ink"
                    }`}
                  >
                    <Icon fontSize="small" className={active ? "text-ink" : "text-ink-3"} />
                    {item.label}
                  </Link>
                );
              })}
            </div>
          ))}
        </nav>
        <div className="border-t border-line p-4">
          <span className="font-mono text-[11px] tracking-wider text-ink-3">
            LOCAL PORTAL
          </span>
        </div>
      </aside>
      <div className="flex min-w-0 flex-1 flex-col">
        <AppHeader />
        <main className="min-w-0 flex-1 overflow-y-auto">{children}</main>
      </div>
    </div>
  );
}
