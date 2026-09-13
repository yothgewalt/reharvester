package harvest

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/yothgewalt/reharvester/internal/paper"
)

// routerRetries is how many times a source inside Router retries a throttled
// request before Router gives up on it: one retry costs 15 s at the default
// delay, where the standalone four cost nearly four minutes.
const routerRetries = 1

// Router is the "auto" source: a failover chain over the networked sources,
// arXiv then OpenAlex then Semantic Scholar.
//
// Every keyword-and-year span goes to the first source still usable. A source
// that fails a span once its retries are spent — throttled, down, rejecting the
// key — is dropped for the rest of the harvest, and the span's remaining quota
// moves to the next source, sharing deduplication with what was already kept.
// A source that answers with fewer records than asked is not failing; its short
// answer stands. Cancellation never switches. When every source is down,
// Harvest returns what it collected with an error naming each failure.
//
// Build one per harvest: the unavailable marks are not safe for concurrent
// harvests and never expire.
type Router struct {
	arxiv *Client
	links []routeLink
}

type routeLink struct {
	name string
	span spanFunc
	// down is why the link was dropped; nil while it is usable.
	down error
}

func newRouter(delay time.Duration, o Options) *Router {
	client := func(d time.Duration) *Client {
		c := NewClient()
		c.Delay, c.MaxRetries = d, routerRetries
		return c
	}
	ax := client(delay)
	oa := &OpenAlex{c: client(delay), Key: keyOr(o.OpenAlexKey, "OPENALEX_API_KEY")}
	s2 := &SemanticScholar{c: client(max(delay, time.Second)), Key: keyOr(o.SemanticScholarKey, "S2_API_KEY")}
	return &Router{arxiv: ax, links: []routeLink{
		{name: SourceArxiv, span: ax.harvestSpan},
		{name: SourceOpenAlex, span: oa.span},
		{name: SourceSemanticScholar, span: s2.span},
	}}
}

func (r *Router) Harvest(ctx context.Context, q Query, prog Progress) ([]paper.Paper, error) {
	return splitHarvest(ctx, q, prog, r.span)
}

func (r *Router) span(ctx context.Context, q Query, max int, seen map[string]struct{}, prog Progress) ([]paper.Paper, error) {
	if seen == nil {
		// Shared down the chain, so a later source skips what an earlier kept.
		seen = make(map[string]struct{}, max)
	}
	var out []paper.Paper
	for i := range r.links {
		l := &r.links[i]
		if l.down != nil {
			continue
		}
		before := len(out)
		got, err := l.span(ctx, q, max-before, seen, func(fetched, total int, msg string) {
			prog(before+fetched, total, l.name+": "+msg)
		})
		out = append(out, got...)
		if err == nil || ctx.Err() != nil {
			return out, err
		}
		l.down = err
		msg := fmt.Sprintf("%s unavailable after %d records (%v) — switching source", l.name, len(got), err)
		log.Printf("harvest: %s", msg)
		prog(len(out), 0, msg)
		if len(out) >= max {
			return out, nil
		}
	}
	return out, r.exhausted()
}

func (r *Router) exhausted() error {
	reasons := make([]string, len(r.links))
	for i, l := range r.links {
		reasons[i] = fmt.Sprintf("%s: %v", l.name, l.down)
	}
	return fmt.Errorf("every source is unavailable — %s", strings.Join(reasons, "; "))
}

// InferCategories probes arXiv while it is usable. When arXiv cannot answer, it
// is dropped from the chain and the harvest goes on unscoped rather than
// failing; keywords arXiv simply has no records for do not drop it.
func (r *Router) InferCategories(ctx context.Context, keywords []string) ([]string, []CategoryProfile, error) {
	if r.links[0].down != nil {
		return nil, nil, nil
	}
	cats, profiles, err := r.arxiv.InferCategories(ctx, keywords)
	switch {
	case err == nil:
		return cats, profiles, nil
	case ctx.Err() != nil:
		return nil, profiles, err
	case errors.Unwrap(err) != nil:
		// A wrapped error is a failed request; a bare one means no keyword
		// matched on arXiv, which says nothing about the other sources.
		r.links[0].down = err
		log.Printf("harvest: arXiv unavailable for category probing (%v) — continuing unscoped with the next source", err)
	default:
		log.Printf("harvest: %v — the other sources may still have them", err)
	}
	return nil, profiles, nil
}
