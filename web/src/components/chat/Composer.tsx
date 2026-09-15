"use client";
import CloseOutlined from "@mui/icons-material/CloseOutlined";
import SendOutlined from "@mui/icons-material/SendOutlined";
import StopCircleOutlined from "@mui/icons-material/StopCircleOutlined";
import IconButton from "@mui/material/IconButton";
import TextField from "@mui/material/TextField";
import { useRef, useState } from "react";

import { MathText } from "@/components/reader/MathText";

const MAX_CHARS = 4000;
const WARN_CHARS = 3800;

export interface ComposerProps {
  onSend(text: string): void;
  disabled: boolean;
  disabledReason?: string;
  streaming: boolean;
  onStop(): void;
  pinnedTitle?: string;
  onUnpin?(): void;
}

/** Send is disabled while `disabled` (no active project / backend offline),
 *  while streaming, or when the trimmed value is empty or over the limit. */
export function Composer({
  onSend,
  disabled,
  disabledReason,
  streaming,
  onStop,
  pinnedTitle,
  onUnpin,
}: ComposerProps) {
  const [value, setValue] = useState("");
  const fieldRef = useRef<HTMLTextAreaElement>(null);
  const trimmed = value.trim();
  const overLimit = value.length > MAX_CHARS;
  const canSend = !disabled && !streaming && trimmed.length > 0 && !overLimit;

  const submit = () => {
    if (!canSend) return;
    onSend(trimmed);
    setValue("");
    fieldRef.current?.focus();
  };

  return (
    <div className="flex flex-col gap-2 border-t border-line p-4">
      {pinnedTitle ? (
        <div className="flex w-fit items-center gap-1.5 rounded-full border border-line py-1 pr-1 pl-3 text-[13px] text-ink-2">
          <span>
            About: <MathText text={pinnedTitle} />
          </span>
          {onUnpin ? (
            <IconButton size="small" aria-label="Remove pinned paper" onClick={onUnpin}>
              <CloseOutlined fontSize="inherit" />
            </IconButton>
          ) : null}
        </div>
      ) : null}
      <form
        className="flex items-end gap-2"
        onSubmit={(e) => {
          e.preventDefault();
          submit();
        }}
      >
        <TextField
          inputRef={fieldRef}
          value={value}
          onChange={(e) => setValue(e.target.value)}
          onKeyDown={(e) => {
            if (e.key !== "Enter" || e.shiftKey || e.nativeEvent.isComposing) return;
            e.preventDefault();
            submit();
          }}
          multiline
          minRows={1}
          maxRows={8}
          fullWidth
          size="small"
          placeholder="Ask about the papers in this project…"
          disabled={disabled}
          slotProps={{ htmlInput: { "aria-label": "Message" } }}
        />
        {streaming ? (
          <IconButton aria-label="Stop generating" onClick={onStop}>
            <StopCircleOutlined />
          </IconButton>
        ) : (
          <IconButton aria-label="Send message" type="submit" disabled={!canSend}>
            <SendOutlined />
          </IconButton>
        )}
      </form>
      <div className="flex min-h-[16px] items-center justify-between gap-3">
        <p className="m-0 text-[13px] text-ink-3">{disabledReason ?? ""}</p>
        {value.length >= WARN_CHARS ? (
          <span className={`font-mono text-[11px] ${overLimit ? "text-error" : "text-warn"}`}>
            {value.length}/{MAX_CHARS}
            {overLimit ? " — too long to send" : ""}
          </span>
        ) : null}
      </div>
    </div>
  );
}
