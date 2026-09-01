/**
 * Splits the markdown from GET /api/v1/wiki/raw/{docId} into the byline and the
 * prose body, so the reader can lay out its own header instead of showing the
 * backend's rendered one.
 *
 * Anchored on two line formats emitted by rag.Render (internal/rag/wiki.go):
 * the "Authors · Year" line and the "Community *x* · PageRank …" metrics line.
 * If either moves, every function here degrades to returning the input
 * unchanged — it never throws and never drops content silently.
 */

const BYLINE = /^(?!#)(.+) · (\d{4})$/;
const METRICS = /^Community \*/;
const NEIGHBOURS = /^## /;

export interface WikiBody {
  /** "Karpukhin et al.", or "" when the authors line is absent. */
  byline: string;
  /** Synthesised intro (when present) plus the abstract. */
  prose: string;
}

export function parseWikiBody(markdown: string): WikiBody {
  const lines = markdown.split("\n");
  const bylineIdx = lines.findIndex((l) => BYLINE.test(l));
  const metricsIdx = lines.findIndex((l) => METRICS.test(l));

  let start = 0;
  let byline = "";
  if (bylineIdx >= 0) {
    byline = shortenAuthors(lines[bylineIdx].replace(BYLINE, "$1"));
    start = bylineIdx + 1;
  } else {
    const h1 = lines.findIndex((l) => l.startsWith("# "));
    start = h1 >= 0 ? h1 + 1 : 0;
  }

  let end = metricsIdx >= 0 ? metricsIdx : lines.findIndex((l, i) => i > start && NEIGHBOURS.test(l));
  if (end < 0) end = lines.length;
  if (end <= start) return { byline, prose: markdown };

  return { byline, prose: lines.slice(start, end).join("\n").trim() };
}

/** "Karpukhin, Oguz, Min" -> "Karpukhin et al."; a lone author keeps its surname. */
function shortenAuthors(authors: string): string {
  const trimmed = authors.replace(/ et al\.$/, "");
  const names = trimmed.split(", ").filter(Boolean);
  if (names.length === 0) return "";
  const parts = names[0].trim().split(/\s+/);
  const surname = parts[parts.length - 1];
  return names.length > 1 || trimmed !== authors ? `${surname} et al.` : surname;
}

const SKIP_LINE = /^\s*(#|-|\||>)/;
const PROTECTED = /\[\[[^\]]*\]\]|`[^`]*`/g;

/**
 * Wraps occurrences of concept-node labels in [[…]] so WikiPane's existing
 * remark-wiki-link resolver turns them into in-page navigation. Only labels
 * that are real nodes are passed in, so every link produced resolves.
 */
export function linkifyConcepts(prose: string, labels: string[], max = 8): string {
  const usable = labels
    .filter((l) => l.length > 3)
    .sort((a, b) => b.length - a.length)
    .slice(0, 200);
  if (usable.length === 0) return prose;

  const pattern = new RegExp(
    `(${PROTECTED.source})|\\b(${usable.map(escapeRe).join("|")})\\b`,
    "gi",
  );

  const seen = new Set<string>();
  return prose
    .split("\n")
    .map((line) => {
      if (SKIP_LINE.test(line)) return line;
      return line.replace(pattern, (whole, protectedRun, term) => {
        if (protectedRun) return whole;
        const key = term.toLowerCase();
        // Once per term: a phrase linked on every occurrence turns the abstract
        // into a wall of links and says nothing extra.
        if (seen.has(key) || seen.size >= max) return whole;
        seen.add(key);
        return `[[${term}]]`;
      });
    })
    .join("\n");
}

function escapeRe(s: string): string {
  return s.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}
