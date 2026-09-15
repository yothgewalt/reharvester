"use client";
import ChatBubbleOutlineOutlined from "@mui/icons-material/ChatBubbleOutlineOutlined";
import Button from "@mui/material/Button";
import Typography from "@mui/material/Typography";
import Link from "next/link";

import { MathText } from "@/components/reader/MathText";
import { projectChatHref } from "@/lib/project-links";
import { useAppStore } from "@/store";
import { useProjectsStore } from "@/store/projects";

/**
 * The reader's way into Chat, pinned to the open paper. With `onAsk` (inside a
 * project page) the button hands the doc id to the caller; without it, it links
 * to the active project's Chat tab and is disabled when no project is active.
 */
export function ChatShortcut({ onAsk }: { onAsk?: (docId: string) => void }) {
  const selectedDocId = useAppStore((s) => s.selectedDocId);
  const nodes = useAppStore((s) => s.nodes);
  const activeProjectId = useProjectsStore((s) => s.projects.find((p) => p.active)?.id);
  const title = nodes.find((n) => n.id === selectedDocId)?.label;
  const icon = <ChatBubbleOutlineOutlined fontSize="small" />;

  let action;
  if (selectedDocId && onAsk) {
    action = (
      <Button variant="contained" startIcon={icon} onClick={() => onAsk(selectedDocId)}>
        Ask about this paper
      </Button>
    );
  } else if (selectedDocId && activeProjectId) {
    action = (
      <Button
        variant="contained"
        startIcon={icon}
        component={Link}
        href={projectChatHref(activeProjectId, selectedDocId)}
      >
        Ask about this paper
      </Button>
    );
  } else {
    action = (
      <Button variant="contained" startIcon={icon} disabled>
        Ask about this paper
      </Button>
    );
  }

  return (
    <div className="flex min-h-0 flex-col gap-4 p-5">
      <Typography variant="overline" id="chat-shortcut-heading" className="text-ink-3">
        Ask about this paper
      </Typography>
      <Typography variant="body2" className="text-ink-2">
        {title ? (
          <>
            Open a chat pinned to <MathText text={title} />, with full history and corpus context.
          </>
        ) : (
          "Select a paper to ask about it in the project's Chat."
        )}
      </Typography>
      {action}
      {!onAsk && activeProjectId ? (
        <Link
          href={projectChatHref(activeProjectId)}
          className="text-[13px] text-accent underline underline-offset-2"
        >
          Open project chat
        </Link>
      ) : null}
    </div>
  );
}
