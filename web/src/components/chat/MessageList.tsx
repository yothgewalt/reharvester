"use client";
import Alert from "@mui/material/Alert";

import { MarkdownProse } from "@/lib/markdown";
import { linkCitations, type ChatMessage } from "@/store/chat-model";
import type { ChatGuard } from "@/types/domain";

import { SourceList } from "./SourceList";

const GUARD_LABEL: Record<ChatGuard, string> = {
  execute: "Read-only",
  "no-match": "Not in this corpus",
};

export function MessageList({ messages }: { messages: ChatMessage[] }) {
  return (
    <ol className="m-0 flex list-none flex-col gap-4 p-0">
      {messages.map((m) =>
        m.role === "user" ? (
          <li key={m.id} className="flex justify-end">
            <p className="m-0 w-fit max-w-[80%] rounded-xl bg-bg-subtle px-4 py-2 text-[14px] whitespace-pre-wrap text-ink">
              {m.content}
            </p>
          </li>
        ) : (
          <li key={m.id} className="flex flex-col gap-2 rounded-xl p-4 ring-line">
            {m.guard ? (
              <span className="w-fit rounded-full px-2 py-0.5 font-mono text-[11px] tracking-wider text-ink-2 uppercase ring-line">
                {GUARD_LABEL[m.guard]}
              </span>
            ) : null}
            {m.content ? (
              <div className="wiki-prose text-[14px]">
                <MarkdownProse>{linkCitations(m.content, m.id, m.sources?.length ?? 0)}</MarkdownProse>
              </div>
            ) : m.status === "streaming" ? (
              <p className="m-0 text-[13px] text-ink-3">The corpus is retrieved — the model is writing.</p>
            ) : null}
            {m.status === "streaming" ? (
              <span className="font-mono text-[11px] text-ink-3">
                writing<span aria-hidden="true">▍</span>
              </span>
            ) : null}
            {m.status === "error" ? <Alert severity="error">{m.error}</Alert> : null}
            {m.sources && m.sources.length > 0 ? <SourceList msgId={m.id} sources={m.sources} /> : null}
          </li>
        ),
      )}
    </ol>
  );
}
