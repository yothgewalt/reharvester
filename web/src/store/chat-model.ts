import type { ChatGuard, ChatRequestMessage, ChatRole, ChatSource } from "@/types/domain";

export interface ChatMessage {
  id: string;
  role: ChatRole;
  content: string;
  sources?: ChatSource[];
  guard?: ChatGuard;
  status: "streaming" | "done" | "error";
  error?: string;
}

export interface Thread {
  id: string;
  title: string;
  createdAt: string;
  pinnedDocId?: string;
  messages: ChatMessage[];
}

const TITLE_MAX = 48;
const HISTORY_MAX = 12;
const CITATION = /\[(\d+)\]/g;

/** A short sidebar label for a thread, derived from its first message. */
export function titleFrom(text: string): string {
  const firstLine = text.trim().split(/\n+/)[0]?.trim() ?? "";
  if (!firstLine) return "New chat";
  return firstLine.length <= TITLE_MAX ? firstLine : `${firstLine.slice(0, TITLE_MAX - 1).trimEnd()}…`;
}

/**
 * The request body's `messages`: the last dozen messages from exchanges that
 * actually finished, in order, ending with the question being sent (pass it as
 * the last element). A question whose reply was a guard notice, an error or a
 * dropped stream is left out along with that reply, so it never re-enters the
 * model as history or lends its words to retrieval.
 */
export function historyFor(messages: ChatMessage[]): ChatRequestMessage[] {
  const answered = (m: ChatMessage | undefined) =>
    m?.role === "assistant" && m.status === "done" && !m.guard;
  return messages
    .filter((m, i) => {
      if (m.role === "assistant") return answered(m);
      return i === messages.length - 1 || answered(messages[i + 1]);
    })
    .slice(-HISTORY_MAX)
    .map((m) => ({ role: m.role, content: m.content }));
}

/**
 * Turns `[n]` citations into anchors on the numbered source cards rendered
 * below the message that produced them (`#src-<msgId>-n`). A number outside
 * 1..count is left as plain text — it did not come from this answer's sources.
 */
export function linkCitations(markdown: string, msgId: string, count: number): string {
  if (count <= 0) return markdown;
  return markdown.replace(CITATION, (match, digits: string) => {
    const n = Number(digits);
    if (!Number.isInteger(n) || n < 1 || n > count) return match;
    return `[${n}](#src-${msgId}-${n})`;
  });
}
