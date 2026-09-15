import type { Metadata } from "next";

import { SettingsSection } from "@/components/settings/SettingsSection";

export const metadata: Metadata = { title: "Settings — Reharvester" };

export default function SettingsPage() {
  return <SettingsSection />;
}
