package app

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/yothgewalt/reharvester/internal/harvest"
	"github.com/yothgewalt/reharvester/internal/store"
)

// Harvest fetches a corpus and writes it to the project. This is the only
// networked operation in the system, and it is bounded by the source API's
// politeness delay rather than by bandwidth.
func Harvest(ctx context.Context, st *store.Store, projectID string, q harvest.Query, delay time.Duration) error {
	if len(q.Categories) == 0 && len(q.Keywords) == 0 {
		return fmt.Errorf("give at least one category or keyword: an unfiltered query would fetch all of arXiv")
	}
	proj, err := st.Project(projectID)
	if err != nil {
		return err
	}
	c := harvest.NewClient()
	c.Delay = delay
	if err := scope(ctx, c, &q); err != nil {
		return err
	}

	start := time.Now()
	papers, err := c.Harvest(ctx, q, func(fetched, total int, msg string) {
		log.Printf("harvest: %d retained (%d match this window) — %s", fetched, total, msg)
	})
	// Harvest returns what it collected alongside any error, and a full harvest
	// is minutes of politeness-limited network. Keep the partial corpus rather
	// than throwing it away on a cancellation or an upstream hiccup.
	if len(papers) == 0 {
		if err != nil {
			return err
		}
		return fmt.Errorf("no records matched")
	}
	if err != nil {
		log.Printf("harvest: interrupted after %d papers (%v) — keeping what was collected", len(papers), err)
	}
	if err := proj.WritePapers(papers); err != nil {
		return err
	}
	// meta.Query is read back as the project's keywords — the "add" action
	// re-harvests from it — so record those in preference to the categories.
	recorded := strings.Join(q.Keywords, ", ")
	if recorded == "" {
		recorded = strings.Join(q.Categories, ", ")
	}
	meta := store.Meta{
		ID: projectID, Name: projectID,
		Query:     recorded,
		CreatedAt: time.Now().UTC(), Status: "complete", DocsIngested: len(papers),
	}
	if err := proj.SaveJSON("meta.json", &meta); err != nil {
		return err
	}
	log.Printf("harvest: %d papers in %s -> %s", len(papers),
		time.Since(start).Round(time.Millisecond), proj.Path("papers.jsonl"))
	return nil
}

// scope fills in the arXiv categories to search by asking arXiv what the
// keywords are about, and reports the reasoning. Categories the caller supplied
// are left alone, so an explicit --categories still pins the scope exactly and
// skips the probe entirely.
func scope(ctx context.Context, c *harvest.Client, q *harvest.Query) error {
	if len(q.Categories) > 0 || len(q.Keywords) == 0 {
		return nil
	}
	log.Printf("harvest: probing arXiv for the categories these keywords belong to")
	cats, profiles, err := c.InferCategories(ctx, q.Keywords)
	for _, p := range profiles {
		log.Printf("harvest: %s", p)
	}
	if err != nil {
		return err
	}
	q.Categories = cats
	if len(cats) == 0 {
		log.Printf("harvest: no clear category emerged — searching all of arXiv")
	} else {
		log.Printf("harvest: scoped to %s", strings.Join(cats, ", "))
	}
	for _, kw := range q.Keywords {
		sub := *q
		sub.Keywords = []string{kw}
		log.Printf("harvest: query %s", sub.SearchQuery())
	}
	return nil
}
