import { create } from "zustand";
import { persist } from "zustand/middleware";

import { ApiError, transport } from "@/lib/api/transport";

import { historyFor, titleFrom, type ChatMessage, type Thread } from "./chat-model";

export type { ChatMessage, Thread } from "./chat-model";

const THREAD_CAP = 30;
const MESSAGE_CAP = 60;

let sendSeq = 0;
let inFlight: AbortController | null = null;

export interface ChatState {
  threadsByProject: Record<string, Thread[]>;
  activeThreadId: Record<string, string | null>;
  /** Creates a thread, prepends it, and makes it the active one for `projectId`. */
  newThread(projectId: string, opts?: { pinnedDocId?: string }): string;
  deleteThread(projectId: string, threadId: string): void;
  selectThread(projectId: string, threadId: string | null): void;
  unpinThread(projectId: string, threadId: string): void;
  /**
   * Appends the user's message and a streaming assistant placeholder, then
   * drives the SSE stream into it. Never rejects — a request or stream
   * failure lands on the assistant message as `status: "error"`. Aborts
   * whatever the store was already sending.
   */
  send(projectId: string, threadId: string, text: string): Promise<void>;
  stop(): void;
}

/** The message to show when a chat request fails before or during streaming. */
export function chatErrorMessage(err: unknown): string {
  if (err instanceof ApiError) {
    const msg = err.fieldErrors?.["messages"] ?? err.fieldErrors?.["_"];
    if (msg) return msg;
    if (err.status === 0) return "Cannot reach the local API.";
  }
  return err instanceof Error ? err.message : "Something went wrong.";
}

function mapThread(
  threads: Record<string, Thread[]>,
  projectId: string,
  threadId: string,
  fn: (t: Thread) => Thread,
): Record<string, Thread[]> {
  const existing = threads[projectId];
  if (!existing) return threads;
  return { ...threads, [projectId]: existing.map((t) => (t.id === threadId ? fn(t) : t)) };
}

function findMessage(
  threads: Record<string, Thread[]>,
  projectId: string,
  threadId: string,
  messageId: string,
): ChatMessage | undefined {
  return threads[projectId]?.find((t) => t.id === threadId)?.messages.find((m) => m.id === messageId);
}

const newId = () => crypto.randomUUID();

export const useChatStore = create<ChatState>()(
  persist(
    (set, get) => ({
      threadsByProject: {},
      activeThreadId: {},

      newThread: (projectId, opts) => {
        const id = newId();
        const thread: Thread = {
          id,
          title: "New chat",
          createdAt: new Date().toISOString(),
          pinnedDocId: opts?.pinnedDocId,
          messages: [],
        };
        set((s) => ({
          threadsByProject: {
            ...s.threadsByProject,
            [projectId]: [thread, ...(s.threadsByProject[projectId] ?? [])].slice(0, THREAD_CAP),
          },
          activeThreadId: { ...s.activeThreadId, [projectId]: id },
        }));
        return id;
      },

      deleteThread: (projectId, threadId) =>
        set((s) => {
          const remaining = (s.threadsByProject[projectId] ?? []).filter((t) => t.id !== threadId);
          const wasActive = s.activeThreadId[projectId] === threadId;
          return {
            threadsByProject: { ...s.threadsByProject, [projectId]: remaining },
            activeThreadId: wasActive
              ? { ...s.activeThreadId, [projectId]: remaining[0]?.id ?? null }
              : s.activeThreadId,
          };
        }),

      selectThread: (projectId, threadId) =>
        set((s) => ({ activeThreadId: { ...s.activeThreadId, [projectId]: threadId } })),

      unpinThread: (projectId, threadId) =>
        set((s) => ({
          threadsByProject: mapThread(s.threadsByProject, projectId, threadId, (t) => ({
            ...t,
            pinnedDocId: undefined,
          })),
        })),

      send: async (projectId, threadId, text) => {
        const question = text.trim();
        if (!question) return;

        // Retrieval takes milliseconds and generation takes seconds, so a
        // second question would otherwise sit behind the first model call.
        inFlight?.abort();
        const controller = new AbortController();
        inFlight = controller;
        const seq = ++sendSeq;

        const thread = get().threadsByProject[projectId]?.find((t) => t.id === threadId);
        if (!thread) return;

        const userMsg: ChatMessage = { id: newId(), role: "user", content: question, status: "done" };
        const assistantMsg: ChatMessage = { id: newId(), role: "assistant", content: "", status: "streaming" };
        const requestMessages = historyFor([...thread.messages, userMsg]);
        const pinnedDocId = thread.pinnedDocId;

        set((s) => ({
          threadsByProject: mapThread(s.threadsByProject, projectId, threadId, (t) => ({
            ...t,
            title: t.messages.length === 0 && !t.pinnedDocId ? titleFrom(question) : t.title,
            messages: [...t.messages, userMsg, assistantMsg].slice(-MESSAGE_CAP),
          })),
        }));

        const patch = (p: Partial<ChatMessage>) => {
          if (seq !== sendSeq) return;
          set((s) => ({
            threadsByProject: mapThread(s.threadsByProject, projectId, threadId, (t) => ({
              ...t,
              messages: t.messages.map((m) => (m.id === assistantMsg.id ? { ...m, ...p } : m)),
            })),
          }));
        };

        try {
          await transport.chatStream(
            { messages: requestMessages, pinnedDocId },
            {
              onSources: (meta) => patch({ sources: meta.sources }),
              onToken: (text) => {
                if (seq !== sendSeq) return;
                const current = findMessage(get().threadsByProject, projectId, threadId, assistantMsg.id);
                patch({ content: (current?.content ?? "") + text });
              },
              onDone: (done) => patch({ content: done.answer, status: "done", guard: done.guard }),
              onError: (message) => patch({ status: "error", error: message }),
            },
            controller.signal,
          );
        } catch (err) {
          patch({ status: "error", error: chatErrorMessage(err) });
        } finally {
          if (inFlight === controller) inFlight = null;
        }
      },

      stop: () => inFlight?.abort(),
    }),
    { name: "reharvester-chat" },
  ),
);
