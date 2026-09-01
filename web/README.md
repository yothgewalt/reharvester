# Reharvester UI

Next.js 16 frontend for the local Reharvester (F-PRD implementation).

## Run

```bash
bun install
bun run dev        # http://localhost:3000
```

There is no mock mode: the UI always talks to the real backend. Start both at once from
the repo root with `./dev.sh -project <id>`, or run the server alone:

```bash
cd .. && go run ./cmd/harvester-server --harvest --categories cs.IR --from 2019 --max 1600
go run ./cmd/harvester-server            # serves :8000
```

`.env.local` only needs `NEXT_PUBLIC_API_BASE_URL`. The backend is a Go single binary —
the "FastAPI" this file used to name never existed. For the neural tier (T3) and wiki synthesis it wants a local
model server: `ollama pull all-minilm` for the 384-d encoder, plus any chat model for
prose. With neither, the system runs at tier T2 and the header says so; nothing errors.

## Stack

Next.js 16 (App Router, Turbopack) · TypeScript 5.9 (pinned — TS7 breaks Next 16.2) ·
MUI v9 + Tailwind v4 (CSS cascade layers, `mui` below `utilities`) · Zustand ·
react-markdown + remark-wiki-link + rehype-raw. No WebGL and no graph-rendering
dependency: the reader is plain DOM and the field map is inline SVG over ~21 nodes,
which is why both stay fast at corpus scale.

Design tokens follow the plane.so taste in the repo-root `CLAUDE.md` (Google Sans 430
headings at LH 1.30/−3% tracking, black CTAs, 1px-ring depth, #0F0F10 canvas slab,
accent #006399 as text only).

## Layout

AppShell: persistent sidebar (Workspace: Harvest `/`, Trends `/trends`, Reader
`/reader`, Projects `/projects`) + header with live crawl chip, status and tier.
The scheduler lives as a tab inside a project, not as its own route. Project detail is
`/projects/detail?id=…` and the reader is `/reader?doc=…` — a static export cannot
pre-render a runtime-minted id, so ids travel in the query string behind a `Suspense`
boundary. Zustand state is client-global, so crawls and selections survive route
changes; `[[wikilinks]]` select in place rather than navigating away.

- **Ingestion** — ≥5 comma-separated keywords or a 100-word abstract gates the harvester;
  crawl progress + terminal log stream over the WebSocket.
- **Trends** — the paper's Fig. 2, in four panels. Top-5 keyword cards by smoothed log₂
  prevalence lift (α = 0.5) against the preceding window, after the specificity filter
  (S ≤ 0.80); a 3–5 yr window slider recalculates. Then **(a)** seven fastest-rising and
  seven steepest-declining bigrams with lift and early → late document counts — a term
  that fell to zero is kept, not filtered out.
  Below them the **coarse-grained field map** (paper Fig. 2c): one SVG node per
  community of 400+ papers, sized by membership, edges for inter-community backbone
  links above threshold, the ten largest labelled. Selecting one opens it in the reader.
  It is deliberately community-level, not paper-level — with 67 communities no
  categorical palette can separate them, but a label can. Finally **(d)** research-gap
  candidates: community pairs ranked by centroid cosine among those with z < −2 against a
  200-permutation null, shown with the observed-vs-null link counts and the share of pairs
  clearing the threshold — the null is weak by construction and the panel says so.
- **Reader** — three panes over one corpus. Left: Louvain communities by size, or paper
  search results. Centre: one paper — byline, year, arXiv category, bridge score, an
  abstract whose concept terms are linkified into `[[wikilinks]]`, and its neighbours
  along the backbone with cosine similarities, descending. Right: ask the corpus a
  question; the context set is shown as seed vs. graph-expansion sources against the
  1500-word budget, so you can see what the answer was built from.
- **Resilience** — offline status chip, LLM-unreachable badge,
  health polling with auto-recovery, and a capability-tier chip (T0–T3) showing which
  rung the backend is running at.

## Verify

```bash
bunx tsc --noEmit && bun run lint && bun run build
bun test src/components
```

Note: the reader parses the markdown from `/api/v1/wiki/raw/{docId}`, anchoring on two
line formats emitted by `rag.Render`. The test above is what guards that coupling; every
parse step degrades to returning the input unchanged rather than throwing.
