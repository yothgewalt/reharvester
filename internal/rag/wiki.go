package rag

import (
	"context"
	"fmt"
	"strings"
)

// WikiLink is one [[wikilink]] target. ID must be a graph node id, because the
// UI resolves a link by slugging its text and matching node ids.
type WikiLink struct {
	ID     string
	Label  string
	Weight float64 // backbone cosine; 0 for concept members, which have no edge
}

// WikiDoc is everything needed to render a reader page. The reader presents
// papers with wiki-style links along backbone edges, so navigation follows
// semantic adjacency rather than a ranked list.
type WikiDoc struct {
	ID        string
	Title     string
	Kind      string // "paper" | "concept"
	Year      int
	Cluster   string
	Abstract  string
	Authors   []string
	Bridge    float64
	PageRank  float64
	Neighbors []WikiLink
	Members   []WikiLink // for a concept: the papers it appears in
}

// Render produces the markdown page with no model involved. This is the T0-T2
// reader: complete, just not synthesised.
func Render(d WikiDoc) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", d.Title)

	if d.Kind == "concept" {
		fmt.Fprintf(&b, "**Concept** in the *%s* community.\n\n", d.Cluster)
		if len(d.Members) > 0 {
			b.WriteString("## Appears in\n\n")
			for _, m := range d.Members {
				fmt.Fprintf(&b, "- [[%s|%s]]\n", m.ID, m.Label)
			}
			b.WriteString("\n")
		}
		return b.String()
	}

	if len(d.Authors) > 0 {
		authors := d.Authors
		suffix := ""
		if len(authors) > 6 {
			authors, suffix = authors[:6], " et al."
		}
		fmt.Fprintf(&b, "%s%s · %d\n\n", strings.Join(authors, ", "), suffix, d.Year)
	}
	if d.Abstract != "" {
		fmt.Fprintf(&b, "%s\n\n", d.Abstract)
	}
	fmt.Fprintf(&b, "Community *%s* · PageRank %.4f · bridge score %.2f\n\n", d.Cluster, d.PageRank, d.Bridge)

	if len(d.Neighbors) > 0 {
		b.WriteString("## Nearest neighbours\n\nAlong the semantic backbone:\n\n")
		for _, n := range d.Neighbors {
			fmt.Fprintf(&b, "- [[%s|%s]] · cos %.2f\n", n.ID, n.Label, n.Weight)
		}
		b.WriteString("\n")
	}
	return b.String()
}

const wikiSystem = `You are annotating one paper inside a local literature graph.
Write two short paragraphs of plain markdown: what the paper does, and how it
relates to its listed neighbours. Do not invent citations, numbers, or results
that are not in the material given. Do not repeat the abstract verbatim. Do not
add a heading. Under 150 words.`

// Synthesize adds a model-written orientation section above the rendered page.
// Any failure returns the rendered page unchanged: the reader is a T0 feature
// and must never depend on a model being present.
func Synthesize(ctx context.Context, o *Ollama, d WikiDoc) string {
	base := Render(d)
	if o == nil || d.Kind != "paper" || d.Abstract == "" {
		return base
	}
	var nb strings.Builder
	for _, n := range d.Neighbors {
		fmt.Fprintf(&nb, "- %s\n", n.Label)
	}
	prompt := fmt.Sprintf("Title: %s\n\nAbstract: %s\n\nCommunity: %s\n\nNeighbours:\n%s",
		d.Title, d.Abstract, d.Cluster, nb.String())
	out, err := o.Generate(ctx, wikiSystem, prompt)
	if err != nil || strings.TrimSpace(out) == "" {
		return base
	}
	head, rest, found := strings.Cut(base, "\n\n")
	if !found {
		return base
	}
	return head + "\n\n" + strings.TrimSpace(out) + "\n\n" + rest
}
