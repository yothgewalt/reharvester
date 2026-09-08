package app

import (
	"log"
	"sort"
	"time"

	"github.com/yothgewalt/reharvester/internal/analyze"
	"github.com/yothgewalt/reharvester/internal/graph"
	"github.com/yothgewalt/reharvester/internal/pipeline"
	"github.com/yothgewalt/reharvester/internal/store"
)

// Analyze reports trends and gap candidates for a built corpus. It rebuilds the
// lexical space and backbone rather than reading the persisted graph, so it can
// run against a corpus that has only been harvested.
func Analyze(st *store.Store, projectID string) error {
	proj, err := st.Project(projectID)
	if err != nil {
		return err
	}
	papers, err := proj.ReadPapers()
	if err != nil {
		return err
	}
	c := pipeline.BuildLexical(papers, pipeline.DefaultBuildOptions())
	g := graph.Build(c.TF, papers, graph.DefaultKNNOptions())

	t := time.Now()
	a := analyze.BuildAnalytics(papers, g.Community, len(g.Communities), 2)
	log.Printf("analyze: %d tracked terms over %v in %s", len(a.Terms), a.Years, time.Since(t).Round(time.Millisecond))

	ys := a.Years
	mid := len(ys) / 2
	early := analyze.Window{From: ys[0], To: ys[mid-1]}
	late := analyze.Window{From: ys[mid], To: ys[len(ys)-1]}
	t = time.Now()
	tr := a.Trends(early, late)
	log.Printf("analyze: early %d-%d vs late %d-%d, %d terms survive specificity, recomputed in %s",
		early.From, early.To, late.From, late.To, len(tr), time.Since(t).Round(time.Microsecond))

	log.Printf("analyze: fastest-rising")
	for i, x := range tr {
		if i >= 7 {
			break
		}
		log.Printf("  %+6.2f log2 lift  %4d late docs  S=%.2f  %s", x.Log2Lift, x.LateDocs, x.Specificity, x.Term)
	}
	log.Printf("analyze: steepest-declining")
	for i := len(tr) - 1; i >= 0 && i > len(tr)-8; i-- {
		x := tr[i]
		log.Printf("  %+6.2f log2 lift  %4d->%4d docs  %s", x.Log2Lift, x.EarlyDocs, x.LateDocs, x.Term)
	}
	byBurst := append([]analyze.Trend(nil), tr...)
	sort.Slice(byBurst, func(i, j int) bool { return byBurst[i].Burst > byBurst[j].Burst })
	log.Printf("analyze: top burst terms (after specificity filter)")
	for i, x := range byBurst {
		if i >= 5 {
			break
		}
		log.Printf("  burst %6.1f  S=%.2f  %s", x.Burst, x.Specificity, x.Term)
	}

	t = time.Now()
	rep := analyze.Gaps(g, analyze.DefaultPermutations)
	log.Printf("analyze: %d/%d community pairs reach z < %.0f in %s",
		rep.BelowThreshold, rep.Total, analyze.GapZMax, time.Since(t).Round(time.Millisecond))
	for i, p := range rep.Pairs {
		if i >= 5 {
			break
		}
		log.Printf("  sim %.3f  %d cross vs null %.0f (z=%.1f)  [%s] <-> [%s]",
			p.Similarity, p.Observed, p.NullMean, p.Z, p.LabelA, p.LabelB)
	}
	return nil
}
