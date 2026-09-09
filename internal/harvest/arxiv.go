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
	HTTP *http.Client
	// BaseURL overrides the arXiv endpoint; empty means the real one. Tests
	// point it at an httptest server.
	BaseURL  string
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

func (c *Client) base() string {
	if c.BaseURL != "" {
		return c.BaseURL
	}
	return endpoint
}

// pause applies the politeness delay before every request after the first.
func (c *Client) pause(ctx context.Context) error {
	if c.warmed {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(c.Delay):
		}
	}
	c.warmed = true
	return nil
}

// Query describes one harvest. Categories and Keywords are OR-ed within
// themselves and AND-ed with each other, so an empty Keywords list means
// "everything in these categories", and an empty Categories list means "every
// arXiv category".
//
// Harvest does not send a multi-keyword Query as one OR-ed request: the
// broadest keyword would take the whole record cap. It gives each keyword its
// own quota instead. See Harvest.
type Query struct {
	Categories []string
	Keywords   []string
	From, To   int // inclusive years; zero means unbounded
	Max        int // hard cap on records fetched
}

// SearchQuery renders the arXiv search_query this Query sends. Exported so a
// harvest can log exactly what it asked for: a corpus that comes back on the
// wrong topic is diagnosed from this string and almost nothing else.
func (q Query) SearchQuery() string {
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
// A multi-keyword query is split one sub-query per keyword, each with an equal
// share of Max. Keywords are never OR-ed into a single request: arXiv sorts by
// date, so the broadest term wins the whole cap. Harvesting "aerodynamic OR
// fighter jet OR space" as one query returns papers about latent spaces and
// nothing about aircraft.
//
// A multi-year span is then harvested year by year with an equal per-year
// quota. That matters for the same reason: a single capped query over 2013-2026
// returns only the newest papers and leaves the trend analysis with nothing to
// compare windows against.
func (c *Client) Harvest(ctx context.Context, q Query, prog Progress) ([]paper.Paper, error) {
	if prog == nil {
		prog = func(int, int, string) {}
	}
	max := q.Max
	if max <= 0 {
		max = 2000
	}
	kws := nonEmpty(q.Keywords)
	if len(kws) <= 1 {
		q.Keywords = kws
		return c.harvestYears(ctx, q, max, nil, prog)
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
		got, err := c.harvestYears(ctx, kq, share, seen, func(fetched, total int, msg string) {
			prog(before+fetched, total, kw+" — "+msg)
		})
		out = append(out, got...)
		prog(len(out), 0, fmt.Sprintf("%s: %d records", kw, len(out)-before))
		if err != nil {
			return out, err
		}
	}
	return out, nil
}

// harvestYears splits one query across its year span. seen may be shared with
// other calls so a record retained under an earlier keyword is not counted
// twice; a nil map means start fresh.
func (c *Client) harvestYears(ctx context.Context, q Query, max int, seen map[string]struct{}, prog Progress) ([]paper.Paper, error) {
	from, to := q.From, q.To
	if from <= 0 || to <= 0 || to < from {
		return c.harvestSpan(ctx, q, max, seen, prog)
	}

	years := to - from + 1
	if years == 1 {
		return c.harvestSpan(ctx, q, max, seen, prog)
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
		if err := c.pause(ctx); err != nil {
			return out, err
		}
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
	v.Set("search_query", q.SearchQuery())
	v.Set("start", fmt.Sprint(start))
	v.Set("max_results", fmt.Sprint(size))
	v.Set("sortBy", "submittedDate")
	v.Set("sortOrder", "descending")
	return c.fetch(ctx, v)
}

// fetch performs one arXiv request and decodes the Atom feed. Callers own the
// politeness delay; see Client.pause.
func (c *Client) fetch(ctx context.Context, v url.Values) (*feed, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base()+"?"+v.Encode(), nil)
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
