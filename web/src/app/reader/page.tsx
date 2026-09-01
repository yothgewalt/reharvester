import type { Metadata } from "next";
import { Suspense } from "react";

import { ReaderSection } from "@/components/reader/ReaderSection";

export const metadata: Metadata = { title: "Reader — Reharvester" };

/**
 * The selected paper travels in the query string (`?doc=`, `?c=`).
 *
 * With `output: "export"` a dynamic segment can only serve ids known at build
 * time, and the Suspense boundary is required because ReaderWorkbench reads
 * useSearchParams — a static export fails to build without it.
 */
export default function ReaderPage() {
  return (
    <Suspense fallback={null}>
      <ReaderSection />
    </Suspense>
  );
}
