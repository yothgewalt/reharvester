"use client";

import FormControl from "@mui/material/FormControl";
import FormHelperText from "@mui/material/FormHelperText";
import InputLabel from "@mui/material/InputLabel";
import MenuItem from "@mui/material/MenuItem";
import Select from "@mui/material/Select";
import TextField from "@mui/material/TextField";
import { useEffect, useRef, useState, type KeyboardEvent } from "react";

import type { FieldErrors } from "@/types/domain";

const DEBOUNCE_MS = 600;

interface AutosaveFieldBase {
  id: string;
  label: string;
  value: string;
  /** Key this field's error, if any, is filed under in the PATCH response. */
  fieldKey: string;
  hint?: string;
  disabled?: boolean;
  /** Sends a PATCH for this field alone; returns the server's field errors, if any. */
  onSave: (value: string) => Promise<FieldErrors | null>;
}

interface TextOrNumberProps extends AutosaveFieldBase {
  type: "text" | "number";
}

interface SelectFieldProps extends AutosaveFieldBase {
  type: "select";
  options: Array<{ value: string; label: string }>;
}

export type AutosaveFieldProps = TextOrNumberProps | SelectFieldProps;

/** Debounced-save state shared by the text/number and select variants below. */
function useAutosaveDraft(
  value: string,
  fieldKey: string,
  onSave: (value: string) => Promise<FieldErrors | null>,
  saveImmediately: boolean,
) {
  const [draft, setDraft] = useState(value);
  const [error, setError] = useState<string | null>(null);
  const dirtyRef = useRef(false);
  const draftRef = useRef(value);
  const savedRef = useRef(value);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    savedRef.current = value;
    if (!dirtyRef.current) setDraft(value);
  }, [value]);

  const commit = async (next: string) => {
    dirtyRef.current = false;
    if (next === savedRef.current) return;
    savedRef.current = next;
    const errors = await onSave(next);
    setError(errors?.[fieldKey] ?? null);
  };

  useEffect(() => {
    return () => {
      if (timerRef.current) clearTimeout(timerRef.current);
      if (dirtyRef.current) void commit(draftRef.current);
    };
    // Flush-on-unmount only; commit/draftRef always read the latest values.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const handleChange = (next: string) => {
    dirtyRef.current = true;
    draftRef.current = next;
    setDraft(next);
    setError(null);
    if (timerRef.current) clearTimeout(timerRef.current);
    if (saveImmediately) {
      void commit(next);
    } else {
      timerRef.current = setTimeout(() => void commit(next), DEBOUNCE_MS);
    }
  };

  const handleBlur = () => {
    if (timerRef.current) clearTimeout(timerRef.current);
    if (dirtyRef.current) void commit(draftRef.current);
  };

  const handleKeyDown = (e: KeyboardEvent) => {
    if (e.key === "Enter") {
      if (timerRef.current) clearTimeout(timerRef.current);
      void commit(draftRef.current);
    }
  };

  return { draft, error, handleChange, handleBlur, handleKeyDown };
}

export function AutosaveField(props: AutosaveFieldProps) {
  const { draft, error, handleChange, handleBlur, handleKeyDown } = useAutosaveDraft(
    props.value,
    props.fieldKey,
    props.onSave,
    props.type === "select",
  );

  if (props.type === "select") {
    const { id, label, hint, disabled, options } = props;
    const helperId = error || hint ? `${id}-helper` : undefined;
    return (
      <FormControl size="small" fullWidth error={Boolean(error)} disabled={disabled}>
        <InputLabel id={`${id}-label`}>{label}</InputLabel>
        <Select
          labelId={`${id}-label`}
          id={id}
          label={label}
          value={draft}
          aria-describedby={helperId}
          onChange={(e) => handleChange(e.target.value)}
        >
          {options.map((opt) => (
            <MenuItem key={opt.value} value={opt.value}>
              {opt.label}
            </MenuItem>
          ))}
        </Select>
        {helperId ? <FormHelperText id={helperId}>{error ?? hint}</FormHelperText> : null}
      </FormControl>
    );
  }

  const { id, label, hint, disabled, type } = props;
  return (
    <TextField
      id={id}
      size="small"
      fullWidth
      type={type}
      label={label}
      value={draft}
      disabled={disabled}
      error={Boolean(error)}
      helperText={error ?? hint}
      onChange={(e) => handleChange(e.target.value)}
      onBlur={handleBlur}
      onKeyDown={handleKeyDown}
    />
  );
}
