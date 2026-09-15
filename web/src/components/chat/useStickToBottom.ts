"use client";
import { useCallback, useLayoutEffect, useRef, useState } from "react";

import { usePrefersReducedMotion } from "@/components/layout/usePrefersReducedMotion";

const NEAR_BOTTOM_PX = 64;

/**
 * Keeps a scroll container pinned to its newest content while the reader is
 * at the bottom, and leaves it alone once they scroll up to read.
 *
 * Attach `ref` and `onScroll` to the scrolling element. `contentKey` must change
 * whenever content grows (e.g. message count plus the last message's length);
 * `resetKey` (e.g. the thread id) jumps back to the bottom when it changes.
 * `atBottom` drives a "jump to latest" control; `scrollToBottom` re-pins, e.g.
 * right after the reader sends a message.
 */
export function useStickToBottom<T extends HTMLElement>(contentKey: string, resetKey: string) {
  const ref = useRef<T>(null);
  const pinned = useRef(true);
  const [atBottom, setAtBottom] = useState(true);
  const reduced = usePrefersReducedMotion();

  const onScroll = useCallback(() => {
    const el = ref.current;
    if (!el) return;
    const near = el.scrollHeight - el.scrollTop - el.clientHeight < NEAR_BOTTOM_PX;
    pinned.current = near;
    setAtBottom(near);
  }, []);

  const scrollToBottom = useCallback(
    (smooth = false) => {
      const el = ref.current;
      if (!el) return;
      pinned.current = true;
      setAtBottom(true);
      el.scrollTo({ top: el.scrollHeight, behavior: smooth && !reduced ? "smooth" : "auto" });
    },
    [reduced],
  );

  useLayoutEffect(() => {
    pinned.current = true;
    scrollToBottom();
  }, [resetKey, scrollToBottom]);

  useLayoutEffect(() => {
    const el = ref.current;
    if (el && pinned.current) el.scrollTop = el.scrollHeight;
  }, [contentKey]);

  return { ref, onScroll, atBottom, scrollToBottom };
}
