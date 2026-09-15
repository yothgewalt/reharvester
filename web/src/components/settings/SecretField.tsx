"use client";

import Button from "@mui/material/Button";
import Chip from "@mui/material/Chip";
import TextField from "@mui/material/TextField";
import { useState, type KeyboardEvent } from "react";

import type { FieldErrors } from "@/types/domain";

interface SecretFieldProps {
  id: string;
  label: string;
  /** Env var the server falls back to when no key is saved, e.g. "OPENALEX_API_KEY". */
  envVar: string;
  hasKey: boolean;
  fieldKey: string;
  disabled?: boolean;
  /** `null` clears the key. Returns the server's field errors, if any. */
  onSave: (value: string | null) => Promise<FieldErrors | null>;
}

export function SecretField({ id, label, envVar, hasKey, fieldKey, disabled, onSave }: SecretFieldProps) {
  const [draft, setDraft] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const commit = async () => {
    const value = draft.trim();
    if (value.length === 0) return;
    setBusy(true);
    const errors = await onSave(value);
    setBusy(false);
    if (errors?.[fieldKey]) {
      setError(errors[fieldKey]);
      return;
    }
    setError(null);
    setDraft("");
  };

  const handleClear = async () => {
    setBusy(true);
    const errors = await onSave(null);
    setBusy(false);
    setError(errors?.[fieldKey] ?? null);
  };

  const handleKeyDown = (e: KeyboardEvent) => {
    if (e.key === "Enter") {
      e.preventDefault();
      void commit();
    }
  };

  return (
    <div className="flex items-end gap-2">
      <TextField
        id={id}
        size="small"
        fullWidth
        type="password"
        autoComplete="off"
        label={label}
        value={draft}
        disabled={disabled || busy}
        error={Boolean(error)}
        helperText={error ?? (hasKey ? "Saved — type to replace" : `Not set (uses ${envVar})`)}
        onChange={(e) => {
          setDraft(e.target.value);
          setError(null);
        }}
        onBlur={() => void commit()}
        onKeyDown={handleKeyDown}
      />
      {hasKey ? (
        <>
          <Chip size="small" variant="outlined" label="Saved" />
          <Button size="small" variant="outlined" disabled={disabled || busy} onClick={() => void handleClear()}>
            {`Clear ${label} key`}
          </Button>
        </>
      ) : null}
    </div>
  );
}
