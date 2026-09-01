# Reharvester — LNCS manuscript (Overleaf package)

Upload this zip directly to Overleaf: **New Project -> Upload Project**.
Then set the compiler and main file if Overleaf does not pick them up automatically:

* Main document: `main.tex`
* Compiler: **pdfLaTeX** (Menu -> Compiler)

Compiles to **12 pages** including references.

## Files

| File | Role |
|---|---|
| `main.tex` | The manuscript |
| `refs.bib` | 23 references; every entry resolved against a live Crossref or arXiv record |
| `llncs.cls` | Springer LNCS class (from CTAN) |
| `splncs04.bst` | Springer LNCS bibliography style (from CTAN) |
| `fig1_overview.pdf` | Fig. 1 — system overview, corpus, build time, capability ladder |
| `fig2_analytics.pdf` | Fig. 2 — phrase trends, trajectories, field map, candidate gaps |
| `fig3_retrieval.pdf` | Fig. 3 — retrieval quality, latency trade-off, context coverage |
| `fig4_scaling.pdf` | Fig. 4 — backbone scaling and the fidelity cost of pruning |

## Local build

    pdflatex main && bibtex main && pdflatex main && pdflatex main

Three pdflatex passes are needed: one to write `.aux`, then bibtex, then two more
to resolve citations and settle float placement.

## Package dependencies beyond a basic TeX Live install

`aliascnt` (required by `llncs.cls`), `xcolor`, `caption`, `titlesec`, `microtype`,
`booktabs`, and `cm-super` (scalable Type 1 fonts, needed because `microtype` font
expansion refuses bitmap fonts). Overleaf's default TeX Live image has all of these.

## Two formatting details worth knowing before you edit

* **Page count is tight.** The document is exactly at the 12-page limit. The line
  `\providecommand{\doi}[1]{}\renewcommand{\doi}[1]{}` just before
  `\bibliographystyle` suppresses DOI URLs in the reference list and is what brings
  the bibliography inside the limit. Removing it adds a page. It must stay *after*
  `\bibliographystyle`, because `splncs04.bst` emits its own `\providecommand{\doi}`
  that would otherwise win.
* **Table 1 is width-critical.** Nine columns at `\tabcolsep=3.0pt` exactly fill the
  text block. Widening the padding or lengthening the `ms/q` header pushes it into
  the right margin (it previously overran by 31pt).
