"use client";
import { useSyncExternalStore } from "react";

const QUERY = "(prefers-reduced-motion: reduce)";

/**
 * Whether the viewer asked the system to reduce motion.
 *
 * Read through useSyncExternalStore rather than an effect that seeds state:
 * matchMedia is an external store, and seeding it with setState inside an
 * effect renders once with the wrong value and then again with the right one.
 * For this hook that means a viewer who asked for no motion sees the animated
 * frame first, which is precisely what they asked to avoid.
 */
export function usePrefersReducedMotion(): boolean {
  return useSyncExternalStore(subscribe, getSnapshot, getServerSnapshot);
}

function subscribe(onStoreChange: () => void): () => void {
  const mq = window.matchMedia(QUERY);
  mq.addEventListener("change", onStoreChange);
  return () => mq.removeEventListener("change", onStoreChange);
}

function getSnapshot(): boolean {
  return window.matchMedia(QUERY).matches;
}

// The UI is a static export, so it is prerendered with no window. React swaps
// in the real value on hydration without reporting a mismatch, which is what
// this third argument exists for.
function getServerSnapshot(): boolean {
  return false;
}
