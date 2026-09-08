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
	proj, err := st.Project(projectID)
	if err != nil {
		return err
	}
	c := harvest.NewClient()
	c.Delay = delay

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
	meta := store.Meta{
		ID: projectID, Name: projectID,
		Query:     strings.Join(q.Categories, ", "),
		CreatedAt: time.Now().UTC(), Status: "complete", DocsIngested: len(papers),
	}
	if err := proj.SaveJSON("meta.json", &meta); err != nil {
		return err
	}
	log.Printf("harvest: %d papers in %s -> %s", len(papers),
		time.Since(start).Round(time.Millisecond), proj.Path("papers.jsonl"))
	return nil
}
