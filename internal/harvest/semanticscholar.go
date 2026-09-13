package harvest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/yothgewalt/reharvester/internal/paper"
)

const (
	s2Endpoint = "https://api.semanticscholar.org/graph/v1/paper/search"
	s2PageSize = 100
	// s2Window is how deep relevance search pages; past it the API refuses.
	s2Window = 1000
)

// SemanticScholar harvests through the Academic Graph relevance search, 100
// records per call and at most the top 1,000 per keyword and year. Relevance,
// not date, orders it: sorted newest-first, any record that mentions a phrase
// once outranks the papers about it. Records with an arXiv id keep it as
// their ID, so they deduplicate against arXiv harvests; the rest are
// "s2:<paperId>". Categories are Semantic Scholar's fields of study, not arXiv
// codes, and Query.Categories is ignored.
type SemanticScholar struct {
	c *Client
	// Key is sent as x-api-key when set.
	Key string
	// BaseURL overrides the endpoint; tests point it at an httptest server.
	BaseURL string
}

type s2Batch struct {
	Next int       `json:"next"`
	Data []s2Paper `json:"data"`
}

type s2Paper struct {
	PaperID         string                  `json:"paperId"`
	ExternalIDs     map[string]any          `json:"externalIds"`
	Title           string                  `json:"title"`
	Abstract        string                  `json:"abstract"`
	Year            int                     `json:"year"`
	PublicationDate string                  `json:"publicationDate"`
	FieldsOfStudy   []string                `json:"fieldsOfStudy"`
	IsOpenAccess    bool                    `json:"isOpenAccess"`
	Authors         []struct{ Name string } `json:"authors"`
}

func (s *SemanticScholar) Harvest(ctx context.Context, q Query, prog Progress) ([]paper.Paper, error) {
	return splitHarvest(ctx, q, prog, tolerate(s.span))
}

func (s *SemanticScholar) span(ctx context.Context, q Query, max int, seen map[string]struct{}, prog Progress) ([]paper.Paper, error) {
	if seen == nil {
		seen = make(map[string]struct{}, max)
	}
	v := url.Values{}
	v.Set("fields", "title,abstract,authors,year,publicationDate,externalIds,fieldsOfStudy,isOpenAccess")
	v.Set("limit", strconv.Itoa(s2PageSize))
	v.Set("offset", "0")
	v.Set("query", strings.Join(nonEmpty(q.Keywords), " "))
	if years := yearRange(q); years != "" {
		v.Set("year", years)
	}
	base := s.BaseURL
	if base == "" {
		base = s2Endpoint
	}

	out := make([]paper.Paper, 0, max)
	fetched := 0
	for len(out) < max {
		if err := s.c.pause(ctx); err != nil {
			return out, err
		}
		body, err := s.c.get(ctx, base+"?"+v.Encode(), withKey("x-api-key", s.Key))
		if err == nil {
			var b s2Batch
			if err = json.Unmarshal(body, &b); err == nil {
				page := make([]paper.Paper, 0, len(b.Data))
				for _, r := range b.Data {
					if p, ok := r.toPaper(); ok {
						page = append(page, p)
					}
				}
				fetched += len(b.Data)
				var kept int
				out, kept = keep(out, seen, max, page)
				prog(len(out), 0, fmt.Sprintf("fetched %d (kept %d)", fetched, kept))
				if b.Next <= 0 || len(b.Data) == 0 || b.Next+s2PageSize > s2Window {
					return out, nil
				}
				v.Set("offset", strconv.Itoa(b.Next))
				continue
			}
			err = fmt.Errorf("semanticscholar: decode: %w", err)
		}
		return out, err
	}
	return out, nil
}

func (r s2Paper) toPaper() (paper.Paper, bool) {
	id := "s2:" + r.PaperID
	if a, ok := r.ExternalIDs["ArXiv"].(string); ok && a != "" {
		id = paper.StripVersion(a)
	}
	published, err := time.Parse(time.DateOnly, r.PublicationDate)
	if err != nil && r.Year > 0 {
		published = time.Date(r.Year, time.January, 1, 0, 0, 0, 0, time.UTC)
	}
	authors := make([]string, len(r.Authors))
	for i, a := range r.Authors {
		authors[i] = a.Name
	}
	return newPaper(id, r.Title, r.Abstract, authors, r.FieldsOfStudy, published, r.IsOpenAccess)
}

// yearRange renders a Query's years as "2024" or "2019-2026", or "" when the
// query is unbounded.
func yearRange(q Query) string {
	switch {
	case q.From <= 0 && q.To <= 0:
		return ""
	case q.From == q.To:
		return strconv.Itoa(q.From)
	case q.From <= 0:
		return "-" + strconv.Itoa(q.To)
	case q.To <= 0:
		return strconv.Itoa(q.From) + "-"
	}
	return fmt.Sprintf("%d-%d", q.From, q.To)
}
