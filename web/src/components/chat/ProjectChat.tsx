"use client";
import ArrowDownwardOutlined from "@mui/icons-material/ArrowDownwardOutlined";
import Button from "@mui/material/Button";
import { useEffect, useRef, useState } from "react";

import { useAppStore } from "@/store";
import { type Thread, useChatStore } from "@/store/chat";
import { useProjectsStore } from "@/store/projects";

import { Composer } from "./Composer";
import { MessageList } from "./MessageList";
import { StarterPrompts } from "./StarterPrompts";
import { ThreadList } from "./ThreadList";
import { useStickToBottom } from "./useStickToBottom";

const NO_THREADS: Thread[] = [];

/**
 * The read-only research chat for one project, as the project page's Chat tab.
 * The caller owns activation: sending stays disabled until `projectId` is the
 * active, loaded corpus. To open a thread pinned to a paper, call the chat
 * store's `newThread(projectId, { pinnedDocId })` before rendering.
 */
export function ProjectChat({ projectId }: { projectId: string }) {
  const project = useProjectsStore((s) => s.projects.find((p) => p.id === projectId));
  const activation = useAppStore((s) => s.activation);
  const backendStatus = useAppStore((s) => s.backendStatus);
  const nodes = useAppStore((s) => s.nodes);

  const threads = useChatStore((s) => s.threadsByProject[projectId] ?? NO_THREADS);
  const activeThreadId = useChatStore((s) => s.activeThreadId[projectId] ?? null);
  const newThread = useChatStore((s) => s.newThread);
  const deleteThread = useChatStore((s) => s.deleteThread);
  const selectThread = useChatStore((s) => s.selectThread);
  const unpinThread = useChatStore((s) => s.unpinThread);
  const send = useChatStore((s) => s.send);
  const stop = useChatStore((s) => s.stop);
  const activeThread = threads.find((t) => t.id === activeThreadId) ?? null;

  const [liveMessage, setLiveMessage] = useState("");
  const lastStatusRef = useRef<string | undefined>(undefined);
  useEffect(() => {
    const last = activeThread?.messages.at(-1);
    if (last?.role === "assistant" && last.status === "done" && lastStatusRef.current === "streaming") {
      setLiveMessage("Answer ready");
    }
    lastStatusRef.current = last?.status;
  }, [activeThread]);

  const projectReady = activation.id === projectId && activation.state === "idle";
  const offline = backendStatus === "offline";
  const composerDisabled = !projectReady || offline;
  const disabledReason = offline
    ? "Backend is offline — reconnect to send messages."
    : projectReady
      ? undefined
      : "Loading this project…";

  const ensureThreadId = () => activeThread?.id ?? newThread(projectId);
  const lastMessage = activeThread?.messages.at(-1);
  const {
    ref: scrollRef,
    onScroll,
    atBottom,
    scrollToBottom,
  } = useStickToBottom<HTMLDivElement>(
    `${activeThread?.messages.length ?? 0}:${lastMessage?.content.length ?? 0}:${lastMessage?.status ?? ""}:${lastMessage?.sources?.length ?? 0}`,
    `${projectId}:${activeThread?.id ?? ""}`,
  );
  const sendAndFollow = (text: string) => {
    void send(projectId, ensureThreadId(), text);
    requestAnimationFrame(() => scrollToBottom());
  };

  const pinnedTitle = activeThread?.pinnedDocId
    ? (nodes.find((n) => n.id === activeThread.pinnedDocId)?.label ?? activeThread.pinnedDocId)
    : undefined;
  const streaming = lastMessage?.status === "streaming";

  return (
    <div className="flex flex-col gap-3">
      <div role="status" className="sr-only">
        {liveMessage}
      </div>
      {project ? (
        <p className="m-0 text-[13px] text-ink-2">
          Answers come only from the {project.docsIngested.toLocaleString()} papers in this project ·
          read-only, can’t run or change anything.
        </p>
      ) : null}

      <div className="grid h-[min(78vh,820px)] min-h-[480px] grid-cols-1 grid-rows-[auto_minmax(0,1fr)] overflow-hidden rounded-xl bg-white ring-line lg:grid-cols-[240px_1px_minmax(0,1fr)] lg:grid-rows-1">
        <nav aria-label="Chat threads" className="max-h-40 min-h-0 overflow-y-auto lg:max-h-none">
          <ThreadList
            threads={threads}
            activeId={activeThreadId}
            onSelect={(id) => selectThread(projectId, id)}
            onNew={() => newThread(projectId)}
            onDelete={(id) => deleteThread(projectId, id)}
          />
        </nav>
        <div className="hidden bg-line lg:block" />
        <div className="flex min-h-0 flex-col">
          <div className="relative min-h-0 flex-1 border-t border-line lg:border-t-0">
            <div
              ref={scrollRef}
              onScroll={onScroll}
              className="h-full overflow-y-auto p-4"
              aria-label="Conversation"
              role="region"
              tabIndex={0}
            >
              {!activeThread || activeThread.messages.length === 0 ? (
                <StarterPrompts disabled={composerDisabled} onPick={sendAndFollow} />
              ) : (
                <MessageList messages={activeThread.messages} />
              )}
            </div>
            {!atBottom && activeThread && activeThread.messages.length > 0 ? (
              <Button
                size="small"
                variant="contained"
                onClick={() => scrollToBottom(true)}
                startIcon={<ArrowDownwardOutlined fontSize="small" />}
                className="!absolute bottom-3 left-1/2 -translate-x-1/2"
              >
                {streaming ? "Jump to latest" : "Back to latest"}
              </Button>
            ) : null}
          </div>
          <Composer
            onSend={sendAndFollow}
            disabled={composerDisabled}
            disabledReason={disabledReason}
            streaming={streaming}
            onStop={stop}
            pinnedTitle={pinnedTitle}
            onUnpin={activeThread ? () => unpinThread(projectId, activeThread.id) : undefined}
          />
        </div>
      </div>
    </div>
  );
}
