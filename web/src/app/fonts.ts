import { IBM_Plex_Mono, Inter } from "next/font/google";
import localFont from "next/font/local";






export const googleSans = localFont({
  src: "./fonts/google-sans-latin.woff2",
  weight: "400 700",
  display: "swap",
  variable: "--font-google-sans",
  adjustFontFallback: false,
});

export const inter = Inter({
  subsets: ["latin"],
  display: "swap",
  variable: "--font-inter",
});

export const plexMono = IBM_Plex_Mono({
  subsets: ["latin"],
  weight: ["400", "500"],
  display: "swap",
  variable: "--font-plex-mono",
});
