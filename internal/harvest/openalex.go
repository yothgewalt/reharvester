package harvest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/yothgewalt/reharvester/internal/paper"
)

const openAlexEndpoint = "https://api.openalex.org/works"

// OpenAlex harvests articles, preprints and reviews by title-and-abstract
// search, 200 per page, most relevant first — sorted by date, the newest
// dataset or poll that mentions a phrase once outranks the papers. Works
// with an arXiv DOI (10.48550/arXiv.*) keep the arXiv id as their ID; the rest
// are "openalex:W…". Categories are the primary topic's subfield then field,
// not arXiv codes, and Query.Categories is ignored.
type OpenAlex struct {
	c *Client
	// Key is sent as api_key when set.
	Key string
	// BaseURL overrides the endpoint; tests point it at an httptest server.
	BaseURL string
}

const openAlexPageSize = 200

type openAlexPage struct {
	Meta struct {
		Count      int    `json:"count"`
		NextCursor string `json:"next_cursor"`
	} `json:"meta"`
	Results []openAlexWork `json:"results"`
}

type openAlexWork struct {
	ID              string           `json:"id"`
	DOI             string           `json:"doi"`
	Title           string           `json:"display_name"`
	PublicationDate string           `json:"publication_date"`
	Abstract        map[string][]int `json:"abstract_inverted_index"`
	Authorships     []struct {
		Author struct {
			Name string `json:"display_name"`
		} `json:"author"`
	} `json:"authorships"`
	PrimaryTopic *struct {
		Subfield struct {
			Name string `json:"display_name"`
		} `json:"subfield"`
		Field struct {
			Name string `json:"display_name"`
		} `json:"field"`
	} `json:"primary_topic"`
	OpenAccess struct {
		IsOA bool `json:"is_oa"`
	} `json:"open_access"`
}

func (o *OpenAlex) Harvest(ctx context.Context, q Query, prog Progress) ([]paper.Paper, error) {
	return splitHarvest(ctx, q, prog, tolerate(o.span))
}

func (o *OpenAlex) span(ctx context.Context, q Query, max int, seen map[string]struct{}, prog Progress) ([]paper.Paper, error) {
	if seen == nil {
		seen = make(map[string]struct{}, max)
	}
	var filters []string
	kws := nonEmpty(q.Keywords)
	if len(kws) > 0 {
		// A comma separates filters, so one cannot appear inside a keyword; the
		// callers already split keywords on commas.
		quoted := make([]string, len(kws))
		for i, k := range kws {
			quoted[i] = strconv.Quote(k)
		}
		filters = append(filters, "title_and_abstract.search:"+strings.Join(quoted, "|"))
	}
	if years := yearRange(q); years != "" {
		filters = append(filters, "publication_year:"+years)
	}
	filters = append(filters, "type:article|preprint|review")
	v := url.Values{}
	v.Set("filter", strings.Join(filters, ","))
	// relevance_score exists only for a search.
	if len(kws) > 0 {
		v.Set("sort", "relevance_score:desc")
	} else {
		v.Set("sort", "publication_date:desc")
	}
	v.Set("per_page", strconv.Itoa(openAlexPageSize))
	v.Set("select", "id,doi,display_name,publication_date,authorships,abstract_inverted_index,primary_topic,open_access")
	v.Set("cursor", "*")
	if o.Key != "" {
		v.Set("api_key", o.Key)
	}
	base := o.BaseURL
	if base == "" {
		base = openAlexEndpoint
	}

	out := make([]paper.Paper, 0, max)
	fetched := 0
	for len(out) < max {
		if err := o.c.pause(ctx); err != nil {
			return out, err
		}
		body, err := o.c.get(ctx, base+"?"+v.Encode(), nil)
		if err == nil {
			var pg openAlexPage
			if err = json.Unmarshal(body, &pg); err == nil {
				page := make([]paper.Paper, 0, len(pg.Results))
				for _, w := range pg.Results {
					if p, ok := w.toPaper(); ok {
						page = append(page, p)
					}
				}
				fetched += len(pg.Results)
				var kept int
				out, kept = keep(out, seen, max, page)
				prog(len(out), pg.Meta.Count, fmt.Sprintf("fetched %d (kept %d)", fetched, kept))
				if pg.Meta.NextCursor == "" || len(pg.Results) == 0 {
					return out, nil
				}
				v.Set("cursor", pg.Meta.NextCursor)
				continue
			}
			err = fmt.Errorf("openalex: decode: %w", err)
		}
		return out, err
	}
	return out, nil
}

const arxivDOIPrefix = "10.48550/arxiv."

func (w openAlexWork) toPaper() (paper.Paper, bool) {
	id := "openalex:" + strings.TrimPrefix(w.ID, "https://openalex.org/")
	doi := strings.ToLower(strings.TrimPrefix(w.DOI, "https://doi.org/"))
	if strings.HasPrefix(doi, arxivDOIPrefix) {
		id = paper.StripVersion(w.DOI[len(w.DOI)-len(doi)+len(arxivDOIPrefix):])
	}
	published, _ := time.Parse(time.DateOnly, w.PublicationDate)
	authors := make([]string, len(w.Authorships))
	for i, a := range w.Authorships {
		authors[i] = a.Author.Name
	}
	var cats []string
	if t := w.PrimaryTopic; t != nil {
		for _, c := range []string{t.Subfield.Name, t.Field.Name} {
			if c != "" {
				cats = append(cats, c)
			}
		}
	}
	return newPaper(id, w.Title, invertedAbstract(w.Abstract), authors, cats, published, w.OpenAccess.IsOA)
}

// invertedAbstract rebuilds the text OpenAlex ships as word -> positions; it
// cannot distribute plain abstracts.
func invertedAbstract(index map[string][]int) string {
	type at struct {
		pos  int
		word string
	}
	var words []at
	for w, ps := range index {
		for _, p := range ps {
			words = append(words, at{p, w})
		}
	}
	sort.Slice(words, func(i, j int) bool { return words[i].pos < words[j].pos })
	parts := make([]string, len(words))
	for i, w := range words {
		parts[i] = w.word
	}
	return strings.Join(parts, " ")
}
