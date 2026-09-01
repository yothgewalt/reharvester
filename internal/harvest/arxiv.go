// Package harvest is the only stage of the pipeline that contacts the network.
// After it has run, every other stage operates on local files.
package harvest

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/yothgewalt/reharvester/internal/paper"
)

const (
	endpoint = "http://export.arxiv.org/api/query"
	// arXiv asks for one request per three seconds. This, not local compute,
	// sets the harvest rate: the paper's 37,006 records took 928 s.
	DefaultDelay    = 3 * time.Second
	DefaultPageSize = 200
	// The paper retains records whose abstract exceeds 200 characters.
	MinAbstractChars = 200
)

type Client struct {
	HTTP     *http.Client
	Delay    time.Duration
	PageSize int

	// warmed records that at least one request has gone out, so the politeness
	// delay also applies between per-year queries, not just between pages.
	warmed bool
}

func NewClient() *Client {
	return &Client{
		HTTP:     &http.Client{Timeout: 60 * time.Second},
		Delay:    DefaultDelay,
		PageSize: DefaultPageSize,
	}
}

// Query describes one harvest. Categories and Keywords are OR-ed within
// themselves and AND-ed with each other, so an empty Keywords list means
// "everything in these categories".
type Query struct {
	Categories []string
	Keywords   []string
	From, To   int // inclusive years; zero means unbounded
	Max        int // hard cap on records fetched
}

func (q Query) searchQuery() string {
	var clauses []string
	if len(q.Categories) > 0 {
		var cats []string
		for _, c := range q.Categories {
			if c = strings.TrimSpace(c); c != "" {
				cats = append(cats, "cat:"+c)
			}
		}
		if len(cats) > 0 {
			clauses = append(clauses, "("+strings.Join(cats, " OR ")+")")
		}
	}
	if len(q.Keywords) > 0 {
		var kws []string
		for _, k := range q.Keywords {
			if k = strings.TrimSpace(k); k != "" {
				kws = append(kws, `all:"`+k+`"`)
			}
		}
		if len(kws) > 0 {
			clauses = append(clauses, "("+strings.Join(kws, " OR ")+")")
		}
	}
	if q.From > 0 || q.To > 0 {
		from, to := q.From, q.To
		if from == 0 {
			from = 1991
		}
		if to == 0 {
			to = time.Now().Year()
		}
		clauses = append(clauses, fmt.Sprintf("submittedDate:[%d01010000 TO %d12312359]", from, to))
	}
	if len(clauses) == 0 {
		return "all:*"
	}
	return strings.Join(clauses, " AND ")
}

// Progress reports harvest state. total is the source's reported match count,
// which may exceed Max.
type Progress func(fetched, total int, msg string)

type feed struct {
	Total   int     `xml:"http://a9.com/-/spec/opensearch/1.1/ totalResults"`
	Entries []entry `xml:"entry"`
}

type entry struct {
	ID        string `xml:"id"`
	Title     string `xml:"title"`
	Summary   string `xml:"summary"`
	Published string `xml:"published"`
	Authors   []struct {
		Name string `xml:"name"`
	} `xml:"author"`
	Primary struct {
		Term string `xml:"term,attr"`
	} `xml:"http://arxiv.org/schemas/atom primary_category"`
	Categories []struct {
		Term string `xml:"term,attr"`
	} `xml:"category"`
}

// Harvest collects records for the query, deduplicating on the version-stripped
// identifier and dropping records whose abstract is at or below
// MinAbstractChars. It returns what it managed to collect even on a mid-stream
// error, so a flaky network degrades the corpus rather than losing it.
//
// A multi-year span is harvested year by year with an equal per-year quota.
// That matters: arXiv sorts by submission date descending, so a single capped
// query over 2013-2026 returns only the newest papers and leaves the trend
// analysis with nothing to compare windows against.
func (c *Client) Harvest(ctx context.Context, q Query, prog Progress) ([]paper.Paper, error) {
	if prog == nil {
		prog = func(int, int, string) {}
	}
	max := q.Max
	if max <= 0 {
		max = 2000
	}
	from, to := q.From, q.To
	if from <= 0 || to <= 0 || to < from {
		return c.harvestSpan(ctx, q, max, nil, prog)
	}

	years := to - from + 1
	if years == 1 {
		return c.harvestSpan(ctx, q, max, nil, prog)
	}
	seen := make(map[string]struct{}, max)
	out := make([]paper.Paper, 0, max)
	for y := to; y >= from; y-- {
		if len(out) >= max {
			break
		}
		// Give every remaining year an equal share of what is still unfilled,
		// so a thin year does not starve the ones after it.
		remainingYears := y - from + 1
		quota := (max - len(out) + remainingYears - 1) / remainingYears
		yq := q
		yq.From, yq.To, yq.Max = y, y, quota
		got, err := c.harvestSpan(ctx, yq, quota, seen, func(fetched, total int, msg string) {
			prog(len(out)+fetched, total, fmt.Sprintf("%d: %s", y, msg))
		})
		out = append(out, got...)
		if err != nil {
			return out, err
		}
	}
	return out, nil
}

// harvestSpan pages one query to a record cap. seen may be shared across calls
// to deduplicate across years; a nil map means start fresh.
func (c *Client) harvestSpan(ctx context.Context, q Query, max int, seen map[string]struct{}, prog Progress) ([]paper.Paper, error) {
	pageSize := c.PageSize
	if pageSize <= 0 {
		pageSize = DefaultPageSize
	}
	if pageSize > max {
		pageSize = max
	}
	if seen == nil {
		seen = make(map[string]struct{}, max)
	}
	out := make([]paper.Paper, 0, max)
	total := 0

	for start := 0; len(out) < max; start += pageSize {
		if start > 0 || c.warmed {
			select {
			case <-ctx.Done():
				return out, ctx.Err()
			case <-time.After(c.Delay):
			}
		}
		c.warmed = true
		f, err := c.page(ctx, q, start, pageSize)
		if err != nil {
			if len(out) > 0 {
				prog(len(out), total, fmt.Sprintf("upstream error after %d records: %v", len(out), err))
				return out, nil
			}
			return nil, err
		}
		if f.Total > 0 {
			total = f.Total
		}
		if len(f.Entries) == 0 {
			break
		}
		kept := 0
		for _, e := range f.Entries {
			p, ok := toPaper(e)
			if !ok {
				continue
			}
			if _, dup := seen[p.ID]; dup {
				continue
			}
			seen[p.ID] = struct{}{}
			out = append(out, p)
			kept++
			if len(out) >= max {
				break
			}
		}
		prog(len(out), total, fmt.Sprintf("fetched %d (kept %d)", start+len(f.Entries), kept))
		if len(f.Entries) < pageSize {
			break
		}
	}
	return out, nil
}

func (c *Client) page(ctx context.Context, q Query, start, size int) (*feed, error) {
	v := url.Values{}
	v.Set("search_query", q.searchQuery())
	v.Set("start", fmt.Sprint(start))
	v.Set("max_results", fmt.Sprint(size))
	v.Set("sortBy", "submittedDate")
	v.Set("sortOrder", "descending")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+v.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "reharvester/0.1 (local-first literature discovery)")
	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		io.Copy(io.Discard, res.Body)
		return nil, fmt.Errorf("arxiv: %s", res.Status)
	}
	var f feed
	if err := xml.NewDecoder(res.Body).Decode(&f); err != nil {
		return nil, fmt.Errorf("arxiv: decode: %w", err)
	}
	return &f, nil
}

func toPaper(e entry) (paper.Paper, bool) {
	abstract := squash(e.Summary)
	if len(abstract) <= MinAbstractChars {
		return paper.Paper{}, false
	}
	title := squash(e.Title)
	if title == "" {
		return paper.Paper{}, false
	}
	published, err := time.Parse(time.RFC3339, e.Published)
	if err != nil {
		return paper.Paper{}, false
	}
	authors := make([]string, 0, len(e.Authors))
	for _, a := range e.Authors {
		if n := squash(a.Name); n != "" {
			authors = append(authors, n)
		}
	}
	// Categories[0] must be the primary category: Task B relevance grading
	// distinguishes the primary from the secondary labels.
	cats := make([]string, 0, len(e.Categories)+1)
	if e.Primary.Term != "" {
		cats = append(cats, e.Primary.Term)
	}
	for _, c := range e.Categories {
		if c.Term != "" && c.Term != e.Primary.Term {
			cats = append(cats, c.Term)
		}
	}
	return paper.Paper{
		ID:         paper.StripVersion(e.ID),
		Title:      title,
		Abstract:   abstract,
		Authors:    authors,
		Categories: cats,
		Published:  published,
		OpenAccess: true, // everything arXiv serves is open access
	}, true
}

// squash collapses the newlines and runs of spaces the arXiv Atom feed wraps
// titles and abstracts with.
func squash(s string) string { return strings.Join(strings.Fields(s), " ") }
