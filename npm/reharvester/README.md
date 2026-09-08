# reharvester

Local-first system for interactive research-literature discovery. Turn a keyword or
abstract query into a navigable map of a research field.

```bash
npm install -g reharvester
reharvester
```

That is the whole install. The package ships a single self-contained binary — the web
interface is compiled into it — so there is no Go toolchain, no Node runtime at run time,
and no build step.

## What you get

`reharvester` with no arguments opens a terminal menu covering the whole workflow:

- **Start server** — serves the API and the web UI on `:8000`
- **Harvest** — fetch a corpus from arXiv, the only step that touches the network
- **Build** — inverted index, TF-IDF, k-NN backbone, communities, trends
- **Console** — live server, request and job output
- **Projects**, **Models**, **Settings**, **Doctor**, **Clean data**

Everything is also scriptable:

```bash
reharvester harvest --project dev --categories cs.IR,cs.DL --from 2019 --max 1600
reharvester build   --project dev
reharvester serve   --project dev
reharvester doctor            # exits non-zero when something is missing
```

## Optional: better retrieval and generated prose

The system runs with nothing installed. Adding a local model server unlocks two tiers:

```bash
ollama pull all-minilm     # 45 MB — dense retrieval, tier T3
ollama pull llama3.2       # generated wiki orientation
```

`reharvester doctor` detects what is missing and can install it for you, showing every
command before it runs.

## Where your data lives

Everything is a file under `.reharvester/` in the working directory — one directory per
project, holding the harvested papers, the graph, the analytics and the embeddings. Pass
`--data <dir>` to put it elsewhere. Nothing leaves your machine except the arXiv harvest.

## Links

- Source and documentation: https://github.com/yothgewalt/reharvester
- Issues: https://github.com/yothgewalt/reharvester/issues
