import type { FieldErrors, HarvestDefaults, HarvestOptionFields } from "@/types/domain";

const MIN_YEAR = 1990;
const MAX_YEAR = 2100;
const MAX_RECORDS = 50000;
const MAX_NAME_LENGTH = 120;
const MAX_CATEGORIES = 16;

/** Merges settings-derived defaults with the fields the user has actually touched. */
export function effectiveOptions(
  defaults: HarvestDefaults,
  overrides: Partial<HarvestOptionFields>,
): HarvestOptionFields {
  return {
    name: overrides.name,
    source: overrides.source ?? defaults.source,
    categories: overrides.categories,
    from: overrides.from ?? defaults.from,
    to: overrides.to ?? defaults.to,
    max: overrides.max ?? defaults.max,
  };
}

/** e.g. "arxiv · 2019–2026 · 2000" or "arxiv · 2019–2026 · 2000 · cs.LG, cs.AI". */
export function summarizeOptions(opts: HarvestOptionFields): string {
  const base = `${opts.source} · ${opts.from}–${opts.to} · ${opts.max}`;
  return opts.categories && opts.categories.length > 0 ? `${base} · ${opts.categories.join(", ")}` : base;
}

export interface ValidateContext {
  sources: readonly string[];
  hasSnapshotPath: boolean;
}

/** Mirrors internal/httpapi/harvest_options.go's resolveHarvest rules client-side. */
export function validateOptions(opts: HarvestOptionFields, ctx: ValidateContext): FieldErrors {
  const errors: FieldErrors = {};

  if (opts.name !== undefined && opts.name.length > MAX_NAME_LENGTH) {
    errors.name = `Name must be ${MAX_NAME_LENGTH} characters or fewer.`;
  }

  if (opts.source !== undefined) {
    if (ctx.sources.length > 0 && !ctx.sources.includes(opts.source)) {
      errors.source = `Unknown source "${opts.source}".`;
    } else if (opts.source === "kaggle" && !ctx.hasSnapshotPath) {
      errors.source = "Set the arXiv snapshot path in Settings first";
    }
  }

  if (opts.from !== undefined) {
    if (!Number.isInteger(opts.from) || opts.from < MIN_YEAR || opts.from > MAX_YEAR) {
      errors.from = `Year must be a whole number between ${MIN_YEAR} and ${MAX_YEAR}.`;
    }
  }
  if (opts.to !== undefined) {
    if (!Number.isInteger(opts.to) || opts.to < MIN_YEAR || opts.to > MAX_YEAR) {
      errors.to = `Year must be a whole number between ${MIN_YEAR} and ${MAX_YEAR}.`;
    }
  }
  if (!errors.from && !errors.to && opts.from !== undefined && opts.to !== undefined && opts.from > opts.to) {
    errors.to = "End year must be on or after the start year.";
  }

  if (opts.max !== undefined && (!Number.isInteger(opts.max) || opts.max < 1 || opts.max > MAX_RECORDS)) {
    errors.max = `Max records must be a whole number between 1 and ${MAX_RECORDS}.`;
  }

  if (opts.categories !== undefined) {
    if (opts.categories.length > MAX_CATEGORIES) {
      errors.categories = `At most ${MAX_CATEGORIES} categories.`;
    } else if (opts.categories.some((c) => /\s/.test(c))) {
      errors.categories = "A category code can't contain spaces.";
    }
  }
  if (!errors.categories && !errors.source && opts.source === "oai" && (opts.categories?.length ?? 0) === 0) {
    errors.categories = "oai requires at least one category.";
  }

  return errors;
}

/** Only the fields the user actually overrode — omitted fields use the server's defaults. */
export function toRequestFields(overrides: Partial<HarvestOptionFields>): HarvestOptionFields {
  const fields: HarvestOptionFields = {};
  if (overrides.name !== undefined && overrides.name.trim().length > 0) fields.name = overrides.name.trim();
  if (overrides.source !== undefined) fields.source = overrides.source;
  if (overrides.categories !== undefined && overrides.categories.length > 0) {
    fields.categories = overrides.categories;
  }
  if (overrides.from !== undefined) fields.from = overrides.from;
  if (overrides.to !== undefined) fields.to = overrides.to;
  if (overrides.max !== undefined) fields.max = overrides.max;
  return fields;
}
