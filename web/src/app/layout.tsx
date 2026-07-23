import { AppRouterCacheProvider } from "@mui/material-nextjs/v16-appRouter";
import type { Metadata } from "next";

import { AppShell } from "@/components/layout/AppShell";

import { googleSans, inter, plexMono } from "./fonts";
import { Providers } from "./providers";

import "./globals.css";

export const metadata: Metadata = {
  title: "Reharvester",
  description:
    "Local research visualization portal — harvest, graph, and read your corpus at localhost.",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html
      lang="en"
      className={`${googleSans.variable} ${inter.variable} ${plexMono.variable} h-full antialiased`}
    >
      <body className="h-full">
        <AppRouterCacheProvider options={{ enableCssLayer: true }}>
          <Providers>
            <AppShell>{children}</AppShell>
          </Providers>
        </AppRouterCacheProvider>
      </body>
    </html>
  );
}
