"use client";
import AddOutlined from "@mui/icons-material/AddOutlined";
import DeleteOutlineOutlined from "@mui/icons-material/DeleteOutlineOutlined";
import IconButton from "@mui/material/IconButton";

import type { Thread } from "@/store/chat-model";

export function ThreadList({
  threads,
  activeId,
  onSelect,
  onNew,
  onDelete,
}: {
  threads: Thread[];
  activeId: string | null;
  onSelect(id: string): void;
  onNew(): void;
  onDelete(id: string): void;
}) {
  return (
    <div className="flex flex-col gap-2 p-3">
      <button
        type="button"
        onClick={onNew}
        className="flex items-center justify-center gap-2 rounded-md bg-slab px-3 py-2 text-[14px] font-semibold text-white transition-colors duration-150 hover:opacity-90"
      >
        <AddOutlined fontSize="small" />
        New chat
      </button>
      {threads.length === 0 ? (
        <p className="m-0 px-1 text-[13px] text-ink-3">No conversations yet.</p>
      ) : (
        <ul className="m-0 flex list-none flex-col gap-1 p-0">
          {threads.map((t) => (
            <li key={t.id} className="flex items-center gap-1">
              <button
                type="button"
                onClick={() => onSelect(t.id)}
                aria-current={t.id === activeId ? "true" : undefined}
                className={`min-w-0 flex-1 truncate rounded-md px-2.5 py-1.5 text-left text-[13px] transition-colors duration-150 ${
                  t.id === activeId ? "bg-bg-subtle font-medium text-ink" : "text-ink-2 hover:bg-bg-faint"
                }`}
              >
                {t.title}
              </button>
              <IconButton size="small" aria-label={`Delete “${t.title}”`} onClick={() => onDelete(t.id)}>
                <DeleteOutlineOutlined fontSize="small" />
              </IconButton>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
