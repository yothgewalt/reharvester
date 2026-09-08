# Reharvester

A local-first system for interactive research-literature discovery, implementing
[`overleaf/main.tex`](overleaf/main.tex).

Turn a keyword or abstract query into a navigable map of a research field. Harvesting
bibliographic records is the only step that touches the network; indexing, graph
construction, trend analysis, retrieval and question answering all run on your machine,
and every intermediate artefact is a file you can open.

## Quick start

```bash
npm install -g reharvester
reharvester
```

That is the whole install. The package ships one self-contained binary per platform with
the web interface compiled in, so it needs no Go toolchain, no Node runtime at run time,
and no build step. `reharvester` with no arguments opens a terminal menu covering the
whole workflow — harvest, build, serve, console, projects, models, reset — and checks the
machine on launch, offering to install anything missing.

Everything the menu does is also a subcommand, so it scripts:

```bash
reharvester harvest --project dev --categories cs.IR,cs.DL --from 2019 --to 2026 --max 1600
reharvester build   --project dev
reharvester serve   --project dev      # API and UI together on :8000
reharvester doctor                     # exits non-zero when something is missing
```

Optional, for the top rung of the ladder and for wiki synthesis:

```bash
ollama pull all-minilm     # 45 MB, 384-d sentence encoder -> tier T3
ollama pull llama3.2       # any chat model -> model-written wiki orientation
```

Neither is required. Without them the system runs at tier T2 and says so. `reharvester
doctor` will install both for you, showing each command before it runs.

### From source

The flag-driven binary is unchanged and remains the documented path for reproducing the
paper's measurements:

```bash
# 1. Harvest a corpus (the only networked step; ~3 s per 200 records by politeness)
go run ./cmd/harvester-server --harvest --project dev \
    --categories cs.IR,cs.DL --from 2019 --to 2026 --max 1600

# 2. Build indexes, backbone and analytics
go run ./cmd/harvester-server --build --project dev

# 3. Serve the API and the UI together (Ctrl-C stops both)
./dev.sh                                            # :8000 and :3000
```

`dev.sh` runs the Next dev server on :3000 for hot reload. A packaged install has no such
process: the UI is static files inside the binary, served by the API itself on :8000.

To build the distributable yourself:

```bash
./scripts/release.sh                   # builds npm/ for six platforms, publishes nothing
```

Supported targets: macOS (arm64, x64), Linux (x64, arm64) and Windows (x64, arm64).
Go commands are scoped to `./cmd/... ./internal/...` rather than `./...`, because a bare
`./...` also compiles a Go file that ships inside an npm dependency under
`web/node_modules`.

### Releasing

Releases run from CI, so any machine can cut one:

```bash
git tag v0.2.0 && git push --tags
```

`.github/workflows/release.yml` builds the UI, cross-compiles all five platforms, runs the
test suite, publishes the platform packages before the root package, and then installs the
result from the registry to prove it works. It needs an `NPM_TOKEN` repository secret
holding an npm **automation** token — a classic token is refused when 2FA is on.

Publishing by hand from a checkout still works, without provenance:

```bash
npm login
./scripts/release.sh --version 0.2.0 --publish
```

## The capability ladder

A local-first system runs on hardware it does not choose, so there is no hard capability
cliff. Four tiers, each a complete system, each adding one dependency:

| Tier | Adds | Needs |
|---|---|---|
| **T0** | inverted index, boolean candidates, token-overlap ranking | nothing beyond the standard library |
| **T1** | TF-IDF vectoriser (unigrams + bigrams, sublinear tf) | — |
| **T2** | mutual k-NN backbone → navigation, communities, trends, gaps | — |
| **T3** | 384-d neural sentence encoder | a local model server |

The tier is reported by `GET /health` and shown in the UI header. Stopping the model
server drops T3 to T2 while the system keeps answering.

## The five stages

Each stage writes a durable artefact under `.reharvester/projects/<id>/`, so any stage can
be re-run, inspected or replaced without re-running the ones before it.

1. **Harvest** — arXiv Atom API, deduplicated on the version-stripped identifier, abstracts
   over 200 characters retained. Multi-year spans are quota'd per year, because arXiv sorts
   by date and a single capped query returns only the newest papers.
2. **Index** — inverted index, TF-IDF (uni + bigrams, sublinear tf, smoothed idf, L2), BM25
   (k1 = 1.5, b = 0.75), and optionally dense. All built over **abstracts only**, which is
   what makes the known-item evaluation a genuine test.
3. **Graph** — mutual k-NN over the TF-IDF space (k = 8, cosine weights), a co-authorship
   layer excluding names on more than 60 papers, Louvain communities labelled by centroid
   terms, PageRank, and a per-node bridge score. Built from the TF-IDF space rather than the
   dense space, so the backbone and everything derived from it survive at T2.
4. **Analyse** — phrase trends by α = 0.5-smoothed log₂ prevalence lift, Kleinberg burst
   detection, a normalised-entropy specificity filter (S ≤ 0.80), and research-gap candidates
   scored against a 200-permutation label null.
5. **Read and ask** — a wiki reader whose `[[wikilinks]]` follow backbone edges, and context
   assembly under a fixed word budget for question answering.

### Two things the analytic layer gets right on purpose

An n-gram extractor that strips stopwords and *then* forms bigrams invents phrases that
never occurred — joining `"…models."` to `"LLMs have…"` across a sentence boundary. The
tokenizer cuts on sentence and clause punctuation first and forms grams only within runs of
adjacent content tokens. LaTeX control words are stripped, or `textbf` ranks as a burst term.
Both failures are pinned by tests in `internal/index/tokenize_test.go`.

## Evaluation

```bash
go run ./cmd/reharvester-eval --project dev --all --out eval.json
```

Reproduces the paper's measurements: Task A (known-item, the query is a title and the answer
is its own abstract), Task B (topical, graded against arXiv's own category labels with
**pooled** judgments across all systems), Task C (context quality against a character-4-gram
gold space no retriever uses), the capability ladder, and end-to-end scaling with log–log
exponents and peak RSS measured one process per corpus size. Confidence intervals are paired
bootstrap at 5,000 resamples.

Measured results against the paper's, on an independently harvested 24,000-paper corpus,
are in [RESULTS.md](RESULTS.md). In short: Table 1's known-item column reproduces to within
a few thousandths, the capability ladder reproduces (T1 and T2 identical, as the paper
argues they should be), and the paper's context-quality negative result reproduces in full.
The paper's *other* negative result — that the exact backbone is quadratic — does not, and
RESULTS.md explains why.

One latency caveat: the dense rows in Table 1 are dominated by a single HTTP round trip per
query to encode it, because the encoder lives in the model server rather than in-process.
The vector scan itself is far cheaper than the sparse cosine it replaces.

Reproducing the *scaling* result needs a corpus of at least ~20,000 papers — the crossover
where inverted-index pruning overtakes the exact backbone. Below that the exact k-NN runs in
milliseconds and the fit measures the clock; the harness says so rather than reporting the
exponent as a finding.

## Layout

```
cmd/harvester-server     harvest, build, serve, and the --graphcheck/--analyze probes
cmd/reharvester-eval     the paper's tables
cmd/ws-probe             end-to-end crawl over the real WebSocket
internal/paper           the record type and the slug rule the whole UI keys on
internal/store           on-disk layout
internal/harvest         arXiv client (the only network client)
internal/index           tokenizer, inverted, TF-IDF, BM25, dense
internal/graph           k-NN backbone (exact and pruned), co-authorship, Louvain, PageRank
internal/analyze         trends, burst, specificity, permutation-null gaps
internal/retrieve        tier selection, RRF fusion, graph expansion
internal/rag             encoder + generation client, wiki synthesis, context assembly
internal/pipeline        the five stages
internal/httpapi         the nine endpoints and the crawl stream
internal/eval            metrics, task construction, paired bootstrap
web/                     Next.js UI (see web/README.md)
```

## Tests

```bash
go test ./...
```

Covers the slug rule against the frontend's regex, the cross-sentence bigram the paper
reports, LaTeX rejection, smoothed lift for a term absent from the early window, specificity
entropy, burst detection, nDCG/precision/recall/MRR, the paired bootstrap, log–log fitting,
cron matching, and the crawl stream's replay-on-reconnect semantics.
