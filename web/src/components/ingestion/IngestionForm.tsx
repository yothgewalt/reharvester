"use client";

import AttachFileOutlined from "@mui/icons-material/AttachFileOutlined";
import CasinoOutlined from "@mui/icons-material/CasinoOutlined";
import Button from "@mui/material/Button";
import Chip from "@mui/material/Chip";
import CircularProgress from "@mui/material/CircularProgress";
import IconButton from "@mui/material/IconButton";
import TextField from "@mui/material/TextField";
import Tooltip from "@mui/material/Tooltip";
import Typography from "@mui/material/Typography";
import { useRef, useState, type DragEvent, type FormEvent } from "react";

import { LlmWarningBadge } from "@/components/system/LlmWarningBadge";
import { useAppStore } from "@/store";
import type { FieldErrors, HarvestOptionFields, IngestPayload } from "@/types/domain";

import { HARVEST_OPTION_FIELD_IDS, HarvestOptions } from "./HarvestOptions";
import { effectiveOptions, validateOptions } from "./harvest-options";
import { randomKeywords } from "./random-keywords";
import { classifyIngestInput } from "./validate";

const ACCEPT = ".pdf,application/pdf";

function isPdf(file: File) {
  return file.type === "application/pdf" || file.name.toLowerCase().endsWith(".pdf");
}

export function IngestionForm() {
  const [raw, setRaw] = useState("");
  const [files, setFiles] = useState<File[]>([]);
  const [dragging, setDragging] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);
  const detailsRef = useRef<HTMLDetailsElement>(null);
  const isCrawlActive = useAppStore((s) => s.isCrawlActive);
  const startCrawl = useAppStore((s) => s.startCrawl);
  const llmStatus = useAppStore((s) => s.llmStatus);
  const settings = useAppStore((s) => s.settings);
  const [randomizing, setRandomizing] = useState(false);
  const [overrides, setOverrides] = useState<Partial<HarvestOptionFields>>({});
  const [serverErrors, setServerErrors] = useState<FieldErrors>({});

  const result = classifyIngestInput(raw, files);

  const effectiveOpts = settings ? effectiveOptions(settings.harvestDefaults, overrides) : null;
  const optionErrors: FieldErrors = effectiveOpts
    ? {
        ...validateOptions(effectiveOpts, {
          sources: settings?.sources ?? [],
          hasSnapshotPath: settings?.hasSnapshotPath ?? true,
        }),
        ...serverErrors,
      }
    : serverErrors;
  const hasOptionErrors = Object.keys(optionErrors).length > 0;

  const handleOptionsChange = (next: Partial<HarvestOptionFields>) => {
    setOverrides(next);
    setServerErrors({});
  };

  const addFiles = (incoming: FileList | null) => {
    if (!incoming) return;
    const newFiles = Array.from(incoming).filter((f) => isPdf(f));
    if (newFiles.length === 0) return;
    setFiles((prev) => [...prev, ...newFiles]);
  };

  const removeFile = (index: number) => {
    setFiles((prev) => prev.filter((_, i) => i !== index));
  };

  const handleAttachClick = () => inputRef.current?.click();

  const randomizingText =
    llmStatus === "ok" ? "Asking the local model for random keywords…" : "Picking random keywords…";

  const handleRandomClick = async () => {
    setRandomizing(true);
    const keywords = await randomKeywords(llmStatus === "ok");
    setFiles([]);
    setRaw(keywords.join(", "));
    setRandomizing(false);
  };

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    addFiles(e.target.files);
    e.target.value = "";
  };

  const handleDragOver = (e: DragEvent<HTMLDivElement>) => {
    e.preventDefault();
    setDragging(true);
  };

  const handleDragLeave = (e: DragEvent<HTMLDivElement>) => {
    e.preventDefault();
    setDragging(false);
  };

  const handleDrop = (e: DragEvent<HTMLDivElement>) => {
    e.preventDefault();
    setDragging(false);
    addFiles(e.dataTransfer.files);
  };

  const submit = async (payload: IngestPayload) => {
    if (detailsRef.current) detailsRef.current.open = false;
    const fieldErrors = await startCrawl(payload, overrides);
    if (!fieldErrors) return;
    setServerErrors(fieldErrors);
    if (detailsRef.current) detailsRef.current.open = true;
    const firstKey = (Object.keys(fieldErrors) as Array<keyof HarvestOptionFields>).find(
      (key) => key in HARVEST_OPTION_FIELD_IDS,
    );
    if (firstKey) document.getElementById(HARVEST_OPTION_FIELD_IDS[firstKey])?.focus();
  };

  const handleSubmit = (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    if (!result.valid || !result.payload || isCrawlActive || hasOptionErrors) return;
    void submit(result.payload);
  };

  return (
    <form onSubmit={handleSubmit} className="flex w-full flex-col gap-2">
      <input
        ref={inputRef}
        type="file"
        accept={ACCEPT}
        multiple
        className="hidden"
        onChange={handleFileChange}
      />
      {}
      <div
        className="relative"
        onDragOver={handleDragOver}
        onDragLeave={handleDragLeave}
        onDrop={handleDrop}
      >
        <TextField
          multiline
          minRows={4}
          fullWidth
          placeholder={
            dragging
              ? "Drop .pdf files here to build a graph"
              : "Enter keywords, paste an abstract, or drop .pdfs…"
          }
          value={raw}
          onChange={(e) => setRaw(e.target.value)}
          disabled={isCrawlActive || randomizing}

          sx={{ "& .MuiOutlinedInput-root": { paddingBottom: files.length > 0 ? "80px" : "60px" } }}
          slotProps={{
            htmlInput: {
              "aria-label": "Keywords, abstract, or PDFs",
              "aria-describedby": "ingest-hint",
            },
          }}
        />
        {dragging && (
          <div className="pointer-events-none absolute inset-0 rounded-md ring-2 ring-accent ring-inset" />
        )}
        <div className="pointer-events-none absolute inset-x-3 bottom-3 flex items-end justify-between gap-3">
          <div className="pointer-events-auto flex flex-wrap items-center gap-2">
            {result.mode !== "empty" && result.mode !== "pdf" ? (
              <Chip
                size="small"
                variant="outlined"
                label={result.mode === "keywords" ? "Keyword mode" : "Abstract mode"}
              />
            ) : null}
            {files.map((file, i) => (
              <Chip
                key={`${file.name}-${i}`}
                size="small"
                variant="outlined"
                color={result.valid ? "default" : "error"}
                label={file.name}
                onDelete={() => removeFile(i)}
              />
            ))}
            {/* PRD: LLM-unreachable warning sits next to the Graph-RAG input. */}
            <LlmWarningBadge />
          </div>
          <div className="pointer-events-auto flex shrink-0 items-center gap-3">
            <Tooltip title={llmStatus === "ok" ? "Random keywords (local model)" : "Random keywords"}>
              <span>
                <IconButton
                  size="small"
                  onClick={() => void handleRandomClick()}
                  disabled={isCrawlActive || randomizing}
                  aria-label="Random keywords"
                  aria-busy={randomizing}
                >
                  {randomizing ? (
                    <CircularProgress size={18} color="inherit" className="text-ink" aria-hidden />
                  ) : (
                    <CasinoOutlined fontSize="small" />
                  )}
                </IconButton>
              </span>
            </Tooltip>
            <Tooltip title="Attach .pdfs">
              <IconButton
                size="small"
                onClick={handleAttachClick}
                disabled={isCrawlActive}
                aria-label="Attach .pdfs"
              >
                <AttachFileOutlined fontSize="small" />
              </IconButton>
            </Tooltip>
            {Number(result.counter) > 0 && (<span className="font-mono text-[13px] text-ink-2">{result.counter}</span>)}
            <Button
              type="submit"
              variant="contained"
              className="py-1.5"
              disabled={!result.valid || isCrawlActive || hasOptionErrors}
            >
              Run Local Harvester
            </Button>
          </div>
        </div>
      </div>
      <Typography id="ingest-hint" variant="caption" className="px-1 text-ink-2">
        {randomizing ? randomizingText : result.hint}
      </Typography>
      <span role="status" className="sr-only">
        {randomizing ? randomizingText : ""}
      </span>
      <HarvestOptions
        overrides={overrides}
        onChange={handleOptionsChange}
        serverErrors={serverErrors}
        disabled={isCrawlActive}
        detailsRef={detailsRef}
      />
    </form>
  );
}
