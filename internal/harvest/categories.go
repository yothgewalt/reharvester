package harvest

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"
)

const (
	// ProbeSize is how many records one keyword's category probe reads.
	ProbeSize = 50
	// MaxInferred caps the inferred category set. Beyond roughly this many the
	// scope stops narrowing anything.
	MaxInferred = 8
	// MinCategoryShare is the fraction of the merged mass a category must hold
	// to make the set. One keyword contributes a mass of 1, so this is "named
	// by at least a twentieth of the keywords that returned anything".
	MinCategoryShare = 0.05
	// MinInferred is the smallest set worth scoping to. Below it InferCategories
	// returns nothing and the caller searches all of arXiv, which is safe
	// because the keywords themselves still filter.
	MinInferred = 2
)

// CategoryProfile is what one keyword's probe found. Total is arXiv's own match
// count for the keyword across all categories and all time, so a Total of 0
// means the keyword is dead and a very large one means it is generic.
type CategoryProfile struct {
	Keyword    string
	Total      int
	Categories []string
}

// InferCategories asks arXiv which categories a keyword set is actually about,
// by probing each keyword on its own and tallying the primary category of the
// most relevant records.
//
// It returns the merged category set, one profile per keyword in the order
// given, and an error only when every keyword came back empty. An empty
// category set is a valid, non-error answer meaning "do not scope this
// harvest"; callers should pass it straight into Query.Categories.
//
// Each keyword contributes equal weight regardless of how many records it
// matches, so a term with 7,000 hits cannot outvote one with 19. Probes are
// relevance-sorted, which matters: "space" by relevance is astrophysics, but
// "space" by date is whatever machine-learning preprint mentioned a latent
// space this morning.
//
// One request per keyword, each subject to the politeness delay.
func (c *Client) InferCategories(ctx context.Context, keywords []string) ([]string, []CategoryProfile, error) {
	kws := nonEmpty(keywords)
	if len(kws) == 0 {
		return nil, nil, nil
	}

	weights := map[string]float64{}
	profiles := make([]CategoryProfile, 0, len(kws))
	live := 0

	for _, kw := range kws {
		if err := c.pause(ctx); err != nil {
			return nil, profiles, err
		}
		f, err := c.probe(ctx, kw)
		if err != nil {
			return nil, profiles, fmt.Errorf("probing %q: %w", kw, err)
		}
		counts := map[string]int{}
		for _, e := range f.Entries {
			if t := strings.TrimSpace(e.Primary.Term); t != "" {
				counts[t]++
			}
		}
		profiles = append(profiles, CategoryProfile{
			Keyword: kw, Total: f.Total, Categories: rank(counts),
		})
		if len(counts) == 0 {
			continue
		}
		live++
		// Normalise to a mass of 1 per keyword before merging.
		seenTotal := 0
		for _, n := range counts {
			seenTotal += n
		}
		for cat, n := range counts {
			weights[cat] += float64(n) / float64(seenTotal)
		}
	}

	if live == 0 {
		return nil, profiles, fmt.Errorf("no arXiv records match any of: %s", strings.Join(kws, ", "))
	}

	floor := MinCategoryShare * float64(live)
	kept := make([]string, 0, len(weights))
	for cat, w := range weights {
		if w >= floor {
			kept = append(kept, cat)
		}
	}
	sort.Slice(kept, func(i, j int) bool {
		if weights[kept[i]] != weights[kept[j]] {
			return weights[kept[i]] > weights[kept[j]]
		}
		return kept[i] < kept[j]
	})
	if len(kept) > MaxInferred {
		kept = kept[:MaxInferred]
	}
	if len(kept) < MinInferred {
		return nil, profiles, nil
	}
	return kept, profiles, nil
}

// String renders one line of harvest log: the evidence behind the inferred
// scope, or the reason a keyword will contribute nothing.
func (p CategoryProfile) String() string {
	if p.Total == 0 || len(p.Categories) == 0 {
		return fmt.Sprintf("%q matches nothing on arXiv and will contribute no papers", p.Keyword)
	}
	top := p.Categories
	if len(top) > 3 {
		top = top[:3]
	}
	return fmt.Sprintf("%q: %d records on arXiv, mostly %s", p.Keyword, p.Total, strings.Join(top, ", "))
}

func (c *Client) probe(ctx context.Context, keyword string) (*feed, error) {
	v := url.Values{}
	v.Set("search_query", Query{Keywords: []string{keyword}}.SearchQuery())
	v.Set("start", "0")
	v.Set("max_results", fmt.Sprint(ProbeSize))
	v.Set("sortBy", "relevance")
	return c.fetch(ctx, v)
}

// rank orders categories by count, descending, breaking ties by name so the
// result does not change between runs on the same input.
func rank(counts map[string]int) []string {
	out := make([]string, 0, len(counts))
	for cat := range counts {
		out = append(out, cat)
	}
	sort.Slice(out, func(i, j int) bool {
		if counts[out[i]] != counts[out[j]] {
			return counts[out[i]] > counts[out[j]]
		}
		return out[i] < out[j]
	})
	return out
}

func nonEmpty(ss []string) []string {
	out := make([]string, 0, len(ss))
	for _, s := range ss {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}
