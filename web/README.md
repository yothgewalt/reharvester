# Reharvester UI

Next.js 16 + Reagraph frontend for the local Reharvester (F-PRD implementation).

## Run

```bash
bun install
bun run dev        # http://localhost:3000 — mock mode by default, no backend needed
```

Mock mode (`NEXT_PUBLIC_USE_MOCKS=true`, set in `.env.development`) ships a seeded
450-node corpus, a simulated ~30s WebSocket crawl stream, wiki fixtures, and trends.
To run against the real FastAPI backend on `localhost:8000`, copy `.env.local.example`
to `.env.local`, set `NEXT_PUBLIC_USE_MOCKS=false`, restart. Without a backend you'll
get the blocking offline overlay (by design).

## Stack

Next.js 16 (App Router, Turbopack) · TypeScript 5.9 (pinned — TS7 breaks Next 16.2) ·
MUI v9 + Tailwind v4 (CSS cascade layers, `mui` below `utilities`) · Zustand ·
Reagraph 4.32.0 (pinned; never add `three` to deps — reagraph bundles its own) ·
react-markdown + remark-wiki-link + rehype-raw.

Design tokens follow the plane.so taste in the repo-root `CLAUDE.md` (Google Sans 430
headings at LH 1.30/−3% tracking, black CTAs, 1px-ring depth, #0F0F10 canvas slab,
accent #006399 as text only).

## Layout

AppShell: persistent sidebar (Workspace: Harvest `/`, Trends `/trends`, Graph `/graph`,
Corpus `/corpus` · Automation: Scheduler `/scheduler`) + header with live crawl chip and
status. Zustand state is client-global, so crawls/selections survive route changes —
wikilinks on `/corpus` navigate to `/graph` and center the selected node.

- **Ingestion** — ≥5 comma-separated keywords or a 100-word abstract gates the harvester;
  crawl progress + terminal log stream over the (mock) WebSocket.
- **Trends** — top-5 regression-slope keywords; 3–5 yr window slider recalculates.
- **Graph** — Reagraph WebGL, pagerank sizing, 5 switchable layouts, 1-hop highlight,
  gaps overlay, `?nodes=10000` stress mode (labels/animations auto-degrade >1.5k/5k).
- **Corpus** — searchable paper list (Open-Access filter) beside an Obsidian-style wiki
  pane; `[[wikilinks]]` select and center the matching graph node.
- **Resilience** — WebGL fallback alert, blocking backend-offline overlay, LLM-unreachable
  badge, health polling with auto-recovery.

## Verify

```bash
bunx tsc --noEmit && bun run build
```

Known quirk: rapid layout switches while a crawl streams graph deltas can log a one-off
reagraph-internal `tick` error (library race, non-fatal).
