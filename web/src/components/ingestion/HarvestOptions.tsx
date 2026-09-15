"use client";

import Button from "@mui/material/Button";
import FormControl from "@mui/material/FormControl";
import FormHelperText from "@mui/material/FormHelperText";
import InputLabel from "@mui/material/InputLabel";
import MenuItem from "@mui/material/MenuItem";
import Select from "@mui/material/Select";
import TextField from "@mui/material/TextField";
import Link from "next/link";
import { useEffect, useState, type Ref } from "react";

import { useAppStore } from "@/store";
import type { FieldErrors, HarvestOptionFields } from "@/types/domain";

import { effectiveOptions, summarizeOptions, validateOptions } from "./harvest-options";

/** Stable ids so a caller can `document.getElementById(...)?.focus()` after a server 400. */
export const HARVEST_OPTION_FIELD_IDS: Record<keyof HarvestOptionFields, string> = {
  name: "harvest-opt-name",
  source: "harvest-opt-source",
  categories: "harvest-opt-categories",
  from: "harvest-opt-from",
  to: "harvest-opt-to",
  max: "harvest-opt-max",
};

interface HarvestOptionsProps {
  overrides: Partial<HarvestOptionFields>;
  onChange: (overrides: Partial<HarvestOptionFields>) => void;
  serverErrors?: FieldErrors;
  disabled?: boolean;
  detailsRef?: Ref<HTMLDetailsElement>;
}

const numberOrUndefined = (raw: string): number | undefined => (raw === "" ? undefined : Number(raw));

export function HarvestOptions({ overrides, onChange, serverErrors, disabled, detailsRef }: HarvestOptionsProps) {
  const settings = useAppStore((s) => s.settings);
  const settingsLoadError = useAppStore((s) => s.settingsLoadError);
  const loadSettings = useAppStore((s) => s.loadSettings);
  const [categoriesRaw, setCategoriesRaw] = useState(() => overrides.categories?.join(", ") ?? "");

  useEffect(() => {
    if (!settings && !settingsLoadError) void loadSettings();
  }, [settings, settingsLoadError, loadSettings]);

  const effective = settings ? effectiveOptions(settings.harvestDefaults, overrides) : null;
  const clientErrors = effective
    ? validateOptions(effective, { sources: settings?.sources ?? [], hasSnapshotPath: settings?.hasSnapshotPath ?? true })
    : {};
  const errors: FieldErrors = { ...clientErrors, ...serverErrors };

  const summaryText = effective ? summarizeOptions(effective) : "Defaults unavailable — the server will use its settings";

  const setCategories = (raw: string) => {
    setCategoriesRaw(raw);
    const parsed = raw
      .split(",")
      .map((c) => c.trim())
      .filter((c) => c.length > 0);
    onChange({ ...overrides, categories: parsed.length > 0 ? parsed : undefined });
  };

  const handleReset = () => {
    setCategoriesRaw("");
    onChange({});
  };

  const showSnapshotHint = errors.source === "Set the arXiv snapshot path in Settings first";

  return (
    <details ref={detailsRef} className="rounded-md border border-line">
      <summary className="cursor-pointer select-none rounded-md px-3 py-2 font-mono text-[13px] uppercase tracking-wide text-ink-2">
        Options <span className="normal-case tracking-normal text-ink-3">— {summaryText}</span>
      </summary>
      <div className="flex flex-col gap-3 border-t border-line px-3 py-3">
        <div className="grid grid-cols-2 gap-3">
          <TextField
            id={HARVEST_OPTION_FIELD_IDS.name}
            size="small"
            label="Project name"
            disabled={disabled}
            error={Boolean(errors.name)}
            helperText={errors.name ?? "Optional — a name is inferred otherwise."}
            value={overrides.name ?? ""}
            onChange={(e) => onChange({ ...overrides, name: e.target.value })}
          />
          {settings && settings.sources.length > 0 ? (
            <FormControl size="small" error={Boolean(errors.source)} disabled={disabled}>
              <InputLabel id={`${HARVEST_OPTION_FIELD_IDS.source}-label`}>Source</InputLabel>
              <Select
                labelId={`${HARVEST_OPTION_FIELD_IDS.source}-label`}
                id={HARVEST_OPTION_FIELD_IDS.source}
                label="Source"
                value={overrides.source ?? settings.harvestDefaults.source}
                aria-describedby={errors.source ? `${HARVEST_OPTION_FIELD_IDS.source}-helper` : undefined}
                onChange={(e) => onChange({ ...overrides, source: e.target.value })}
              >
                {settings.sources.map((src) => (
                  <MenuItem key={src} value={src}>
                    {src}
                  </MenuItem>
                ))}
              </Select>
              {errors.source ? (
                <FormHelperText id={`${HARVEST_OPTION_FIELD_IDS.source}-helper`}>
                  {errors.source}
                  {showSnapshotHint ? (
                    <>
                      {" "}
                      <Link href="/settings" className="text-accent underline">
                        Open Settings
                      </Link>
                    </>
                  ) : null}
                </FormHelperText>
              ) : null}
            </FormControl>
          ) : (
            <TextField
              id={HARVEST_OPTION_FIELD_IDS.source}
              size="small"
              label="Source"
              disabled={disabled}
              error={Boolean(errors.source)}
              helperText={errors.source ?? "Blank uses the server's configured source."}
              value={overrides.source ?? ""}
              onChange={(e) => onChange({ ...overrides, source: e.target.value || undefined })}
            />
          )}
          <TextField
            id={HARVEST_OPTION_FIELD_IDS.categories}
            size="small"
            className="col-span-2"
            label="Categories (comma-separated)"
            disabled={disabled}
            error={Boolean(errors.categories)}
            helperText={errors.categories ?? "Blank infers them on arXiv; oai requires them."}
            value={categoriesRaw}
            onChange={(e) => setCategories(e.target.value)}
          />
          <TextField
            id={HARVEST_OPTION_FIELD_IDS.from}
            size="small"
            type="number"
            label="From year"
            disabled={disabled}
            error={Boolean(errors.from)}
            helperText={errors.from}
            value={overrides.from ?? ""}
            onChange={(e) => onChange({ ...overrides, from: numberOrUndefined(e.target.value) })}
          />
          <TextField
            id={HARVEST_OPTION_FIELD_IDS.to}
            size="small"
            type="number"
            label="To year"
            disabled={disabled}
            error={Boolean(errors.to)}
            helperText={errors.to}
            value={overrides.to ?? ""}
            onChange={(e) => onChange({ ...overrides, to: numberOrUndefined(e.target.value) })}
          />
          <TextField
            id={HARVEST_OPTION_FIELD_IDS.max}
            size="small"
            type="number"
            label="Max records"
            disabled={disabled}
            error={Boolean(errors.max)}
            helperText={errors.max}
            value={overrides.max ?? ""}
            onChange={(e) => onChange({ ...overrides, max: numberOrUndefined(e.target.value) })}
          />
        </div>
        <Button variant="text" className="self-start" onClick={handleReset} disabled={disabled}>
          Reset to defaults
        </Button>
      </div>
    </details>
  );
}
