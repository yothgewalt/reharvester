

import type { IngestPayload } from "@/types/domain";

export type IngestMode = "keywords" | "abstract" | "pdf" | "empty";

export interface IngestClassification {
  mode: IngestMode;
  valid: boolean;
  counter: string;
  hint: string;
  payload?: IngestPayload;
}

const KEYWORD_MIN = 5;
const ABSTRACT_MIN_WORDS = 100;
const KEYWORD_MAX_WORDS = 6;
const MAX_PDF_FILES = 5;
const MAX_PDF_BYTES_PER_FILE = 20 * 1024 * 1024;
const MAX_PDF_BYTES_TOTAL = 50 * 1024 * 1024;

const EMPTY_HINT =
  "Enter at least 5 comma-separated keywords, paste an abstract of 100+ words, or drop up to 5 .pdfs.";

const countWords = (s: string): number => s.split(/\s+/).filter(Boolean).length;

const formatSize = (bytes: number) =>
  bytes >= 1024 * 1024 ? `${(bytes / (1024 * 1024)).toFixed(1)} MB` : `${Math.round(bytes / 1024)} KB`;


export function classifyIngestInput(raw: string, files: File[] = []): IngestClassification {
  const trimmed = raw.trim();

  if (files.length > 0) {
    if (files.length > MAX_PDF_FILES) {
      return {
        mode: "pdf",
        valid: false,
        counter: `${files.length} / ${MAX_PDF_FILES} files`,
        hint: `Attach at most ${MAX_PDF_FILES} PDF files.`,
        payload: { mode: "pdf", files },
      };
    }

    const nonPdf = files.find((f) => f.type !== "application/pdf" && !f.name.toLowerCase().endsWith(".pdf"));
    if (nonPdf) {
      return {
        mode: "pdf",
        valid: false,
        counter: `${files.length} files`,
        hint: `${nonPdf.name} is not a .pdf — only PDF files are supported.`,
        payload: { mode: "pdf", files },
      };
    }

    const oversized = files.find((f) => f.size > MAX_PDF_BYTES_PER_FILE);
    if (oversized) {
      return {
        mode: "pdf",
        valid: false,
        counter: `${files.length} files`,
        hint: `${oversized.name} is ${formatSize(oversized.size)} — each PDF must be ≤ ${formatSize(
          MAX_PDF_BYTES_PER_FILE,
        )}.`,
        payload: { mode: "pdf", files },
      };
    }

    const totalBytes = files.reduce((sum, f) => sum + f.size, 0);
    if (totalBytes > MAX_PDF_BYTES_TOTAL) {
      return {
        mode: "pdf",
        valid: false,
        counter: `${formatSize(totalBytes)} total`,
        hint: `Total upload is ${formatSize(totalBytes)} — must be ≤ ${formatSize(MAX_PDF_BYTES_TOTAL)}.`,
        payload: { mode: "pdf", files },
      };
    }

    return {
      mode: "pdf",
      valid: true,
      counter: `${files.length} file${files.length === 1 ? "" : "s"}`,
      hint: `${files.length} PDF${files.length === 1 ? "" : "s"} attached — ready to build graph.`,
      payload: { mode: "pdf", files },
    };
  }

  if (trimmed.length === 0) {
    return { mode: "empty", valid: false, counter: "", hint: EMPTY_HINT };
  }

  const segments = trimmed
    .split(",")
    .map((s) => s.trim())
    .filter((s) => s.length > 0);


  const keywordMode = trimmed.includes(",")
    ? segments.length >= 1 && segments.every((s) => countWords(s) <= KEYWORD_MAX_WORDS)
    : countWords(trimmed) <= KEYWORD_MAX_WORDS;

  if (keywordMode) {
    const count = segments.length;
    const valid = count >= KEYWORD_MIN;
    const counter = `${count} / ${KEYWORD_MIN} keywords`;
    if (valid) {
      return {
        mode: "keywords",
        valid,
        counter,
        hint: "Keyword mode. Ready to crawl.",
        payload: { mode: "keywords", keywords: segments },
      };
    }
    const missing = KEYWORD_MIN - count;
    return {
      mode: "keywords",
      valid,
      counter,
      hint: `Keyword mode — add ${missing} more comma-separated keyword(s).`,
    };
  }

  const words = countWords(trimmed);
  const valid = words >= ABSTRACT_MIN_WORDS;
  const counter = `${words} / ${ABSTRACT_MIN_WORDS} words`;
  if (valid) {
    return {
      mode: "abstract",
      valid,
      counter,
      hint: "Abstract mode. Ready to crawl.",
      payload: { mode: "abstract", abstract: trimmed },
    };
  }
  const missing = ABSTRACT_MIN_WORDS - words;
  return {
    mode: "abstract",
    valid,
    counter,
    hint: `Abstract mode — add ${missing} more word(s).`,
  };
}
