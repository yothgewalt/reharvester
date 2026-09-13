package harvest

import (
	"context"
	"fmt"
	"time"

	"github.com/yothgewalt/reharvester/internal/paper"
)

// spanFunc fetches one keyword over one year span to a record cap. seen is
// shared across calls so a record retained earlier is not counted twice; a nil
// map means start fresh. It should return what it collected even on error.
type spanFunc func(ctx context.Context, q Query, max int, seen map[string]struct{}, prog Progress) ([]paper.Paper, error)

// splitHarvest is how every networked source spends Query.Max.
//
// A multi-keyword query is split one sub-query per keyword, each with an equal
// share of Max. A multi-year span is then harvested year by year with an equal
// per-year quota: a single capped query over 2013-2026 returns only the newest
// papers and leaves the trend analysis with nothing to compare windows against.
func splitHarvest(ctx context.Context, q Query, prog Progress, span spanFunc) ([]paper.Paper, error) {
	if prog == nil {
		prog = func(int, int, string) {}
	}
	max := q.Max
	if max <= 0 {
		max = DefaultMax
	}
	kws := nonEmpty(q.Keywords)
	if len(kws) <= 1 {
		q.Keywords = kws
		return splitYears(ctx, q, max, nil, prog, span)
	}

	// Each keyword keeps its own share whether or not it fills it. Unlike the
	// per-year quota below, slack is deliberately NOT passed to the keywords
	// that follow: handing it on is how one broad term takes the corpus while
	// the specific ones contribute nothing. A thin keyword set therefore
	// yields fewer than Max records, which is the honest answer.
	share := (max + len(kws) - 1) / len(kws)
	seen := make(map[string]struct{}, max)
	out := make([]paper.Paper, 0, max)
	for _, kw := range kws {
		kq := q
		kq.Keywords, kq.Max = []string{kw}, share
		before := len(out)
		got, err := splitYears(ctx, kq, share, seen, func(fetched, total int, msg string) {
			prog(before+fetched, total, kw+" — "+msg)
		}, span)
		out = append(out, got...)
		prog(len(out), 0, fmt.Sprintf("%s: %d records", kw, len(out)-before))
		if err != nil {
			return out, err
		}
	}
	return out, nil
}

// tolerate turns a span that failed after collecting something into a short
// span, so a standalone source degrades to a smaller corpus instead of stopping
// the harvest. Router uses spans without it: there a failure switches source.
func tolerate(span spanFunc) spanFunc {
	return func(ctx context.Context, q Query, max int, seen map[string]struct{}, prog Progress) ([]paper.Paper, error) {
		out, err := span(ctx, q, max, seen, prog)
		if err != nil && len(out) > 0 && ctx.Err() == nil {
			prog(len(out), 0, fmt.Sprintf("upstream error after %d records: %v", len(out), err))
			return out, nil
		}
		return out, err
	}
}

func splitYears(ctx context.Context, q Query, max int, seen map[string]struct{}, prog Progress, span spanFunc) ([]paper.Paper, error) {
	from, to := q.From, q.To
	if from <= 0 || to <= 0 || to <= from {
		return span(ctx, q, max, seen, prog)
	}
	if seen == nil {
		seen = make(map[string]struct{}, max)
	}
	out := make([]paper.Paper, 0, max)
	for y := to; y >= from; y-- {
		if len(out) >= max {
			break
		}
		// Give every remaining year an equal share of what is still unfilled,
		// so a thin year does not starve the ones after it.
		quota := yearQuota(max-len(out), y-from+1)
		yq := q
		yq.From, yq.To, yq.Max = y, y, quota
		got, err := span(ctx, yq, quota, seen, func(fetched, total int, msg string) {
			prog(len(out)+fetched, total, fmt.Sprintf("%d: %s", y, msg))
		})
		out = append(out, got...)
		if err != nil {
			return out, err
		}
	}
	return out, nil
}

func yearQuota(unfilled, remainingYears int) int {
	return (unfilled + remainingYears - 1) / remainingYears
}

// keep appends the records of one page to out, up to max, skipping any whose
// ID or title slug is already in seen. The slug matters because it is a paper's
// node id in the graph and wiki: aggregators list one work under several ids.
// It reports how many it kept.
func keep(out []paper.Paper, seen map[string]struct{}, max int, page []paper.Paper) ([]paper.Paper, int) {
	kept := 0
	for _, p := range page {
		if len(out) >= max {
			break
		}
		if _, dup := seen[p.ID]; dup {
			continue
		}
		// A title with no ASCII letters or digits slugs to "", which must not
		// make every such paper a duplicate of the first.
		slug := paper.Slug(p.Title)
		if slug != "" {
			if _, dup := seen["title:"+slug]; dup {
				continue
			}
			seen["title:"+slug] = struct{}{}
		}
		seen[p.ID] = struct{}{}
		out = append(out, p)
		kept++
	}
	return out, kept
}

// newPaper normalises one record from any source, rejecting it when the title
// is empty or the abstract is at or below MinAbstractChars — the retention
// rule the paper's corpus used. categories[0] must be the source's primary
// label: Task B relevance grading and the graph colouring read it.
func newPaper(id, title, abstract string, authors, categories []string, published time.Time, openAccess bool) (paper.Paper, bool) {
	abstract, title = squash(abstract), squash(title)
	if id == "" || title == "" || len(abstract) <= MinAbstractChars || published.IsZero() {
		return paper.Paper{}, false
	}
	names := make([]string, 0, len(authors))
	for _, a := range authors {
		if a = squash(a); a != "" {
			names = append(names, a)
		}
	}
	return paper.Paper{
		ID: id, Title: title, Abstract: abstract, Authors: names,
		Categories: categories, Published: published, OpenAccess: openAccess,
	}, true
}
