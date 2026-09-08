"use client";
import Alert from "@mui/material/Alert";
import Button from "@mui/material/Button";
import TextField from "@mui/material/TextField";
import Typography from "@mui/material/Typography";

import { useAppStore } from "@/store";
import type { AskSource } from "@/types/domain";

export function AskRail() {
  const question = useAppStore((s) => s.askQuestion);
  const setQuestion = useAppStore((s) => s.setAskQuestion);
  const submitAsk = useAppStore((s) => s.submitAsk);
  const loading = useAppStore((s) => s.askLoading);
  const result = useAppStore((s) => s.askResult);
  const error = useAppStore((s) => s.askError);
  const llmStatus = useAppStore((s) => s.llmStatus);
  const selectNode = useAppStore((s) => s.selectNode);

  const seeds = result?.sources.filter((s) => !s.viaGraph).length ?? 0;
  const expansions = result?.sources.filter((s) => s.viaGraph).length ?? 0;
  const used = result?.sources.reduce((n, s) => n + s.words, 0) ?? 0;
  const grounded = llmStatus === "ok";

  return (
    <div className="flex min-h-0 flex-col gap-4 p-5">
      <Typography variant="overline" id="ask-heading" className="text-ink-3">
        Ask — runs on this machine
      </Typography>

      <form
        className="flex flex-col gap-3"
        onSubmit={(e) => {
          e.preventDefault();
          void submitAsk();
        }}
      >
        <TextField
          size="small"
          multiline
          minRows={2}
          value={question}
          onChange={(e) => setQuestion(e.target.value)}
          placeholder="How did dense retrieval displace BM25?"
          slotProps={{ htmlInput: { "aria-labelledby": "ask-heading" } }}
          fullWidth
        />
        <Button type="submit" variant="contained" disabled={loading || !question.trim()}>
          {loading ? "Assembling…" : "Ask"}
        </Button>
      </form>

      {error ? <Alert severity="error">{error}</Alert> : null}

      {result ? (
        <>
          <div className="flex flex-col gap-2">
            <Typography variant="overline" className="text-ink-3">
              Context set · {result.budget.toLocaleString()}-word budget
            </Typography>
            <div
              className="h-1 w-full overflow-hidden rounded-full bg-bg-subtle"
              role="img"
              aria-label={`${used.toLocaleString()} of ${result.budget.toLocaleString()} words used by ${result.sources.length} sources`}
            >
              <div
                className="h-full bg-slab"
                style={{ width: `${Math.min(100, (used / result.budget) * 100)}%` }}
              />
            </div>
            <p className="m-0 font-mono text-[11px] text-ink-3">
              {seeds} seed · {expansions} expansion ({result.hops}-hop backbone)
            </p>
          </div>

          <ul className="m-0 flex max-h-64 list-none flex-col gap-1 overflow-y-auto p-0">
            {result.sources.map((s, i) => (
              <SourceRow key={`${s.docId}-${i}`} source={s} onOpen={selectNode} />
            ))}
          </ul>

          <div
            role="status"
            aria-live="polite"
            aria-busy={loading}
            className="flex flex-col gap-2 rounded-xl p-4 ring-line"
          >
            <Typography variant="subtitle2" component="h3">
              {grounded ? "Grounded answer" : "Context set only"}
            </Typography>
            <p className="m-0 text-[13px] leading-6 text-ink-2">{result.answer}</p>
          </div>
        </>
      ) : null}
    </div>
  );
}

function SourceRow({
  source,
  onOpen,
}: {
  source: AskSource;
  onOpen(nodeId: string, docId: string): void;
}) {
  const tag = source.viaGraph ? "expansion" : "seed";
  const dot = source.viaGraph ? "bg-ok" : "bg-ink-3";
  const inner = (
    <>
      <span className="flex min-w-0 items-center gap-2">
        <span className={`h-1.5 w-1.5 shrink-0 rounded-full ${dot}`} aria-hidden="true" />
        <span className="truncate">{source.title}</span>
      </span>
      <span className="shrink-0 font-mono text-[11px] text-ink-3">
        <span className="sr-only">{tag}, </span>
        {source.words} w
      </span>
    </>
  );

  // Sources outside the graph snapshot have no node to open.
  if (!source.docId) {
    return (
      <li
        className="flex items-center justify-between gap-3 px-2 py-1.5 text-[13px] text-ink-3"
        title="Outside the graph snapshot"
      >
        {inner}
      </li>
    );
  }

  return (
    <li className="contents">
      <button
        type="button"
        onClick={() => onOpen(source.docId, source.docId)}
        className="flex w-full items-center justify-between gap-3 rounded-md px-2 py-1.5 text-left text-[13px] text-ink-1 transition-colors hover:bg-bg-subtle"
      >
        {inner}
      </button>
    </li>
  );
}
