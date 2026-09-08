// Command reharvester-eval reproduces the paper's measurements over a local
// corpus: retrieval on two tasks with independent ground truth, context quality
// at a fixed budget, the capability ladder, and end-to-end scaling.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/yothgewalt/reharvester/internal/analyze"
	"github.com/yothgewalt/reharvester/internal/eval"
	"github.com/yothgewalt/reharvester/internal/graph"
	"github.com/yothgewalt/reharvester/internal/index"
	"github.com/yothgewalt/reharvester/internal/pipeline"
	"github.com/yothgewalt/reharvester/internal/rag"
	"github.com/yothgewalt/reharvester/internal/retrieve"
	"github.com/yothgewalt/reharvester/internal/store"
)

func main() {
	var (
		data       = flag.String("data", ".reharvester", "directory holding projects and artefacts")
		project    = flag.String("project", "dev", "project to evaluate")
		all        = flag.Bool("all", false, "run every experiment")
		doRetrieve = flag.Bool("retrieval", false, "Task A and Task B (Table 1)")
		doContext  = flag.Bool("context", false, "Task C, context quality at fixed budget")
		doLadder   = flag.Bool("ladder", false, "the capability ladder (Table 2a)")
		doScaling  = flag.Bool("scaling", false, "end-to-end scaling with exponents (Table 2b)")
		scalingOne = flag.Int("scaling-one", 0, "internal: build one corpus size and report timings plus peak RSS")
		nA         = flag.Int("queries-a", 500, "Task A query count")
		nB         = flag.Int("queries-b", 400, "Task B query count")
		nC         = flag.Int("seeds-c", 250, "Task C seed count")
		out        = flag.String("out", "", "write results as JSON to this path")
		ollamaURL  = flag.String("ollama", rag.DefaultBaseURL, "model server for the dense tier")
		embedModel = flag.String("embed-model", rag.DefaultEmbedding, "sentence encoder")
	)
	flag.Parse()

	st, err := store.Open(*data)
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	sp, err := st.Project(*project)
	if err != nil {
		log.Fatalf("project: %v", err)
	}

	// The scaling sweep re-execs this binary once per corpus size so peak
	// resident memory is measured in a fresh process rather than accumulating.
	if *scalingOne > 0 {
		runScalingOne(sp, *scalingOne)
		return
	}

	ctx := context.Background()
	var emb index.Embedder
	o := rag.NewOllama(*ollamaURL, *embedModel, "")
	if o.Available(ctx) && o.HasModel(ctx, *embedModel) {
		emb = o
	} else {
		log.Printf("eval: no encoder available — the dense rows will be omitted and the ladder stops at T2")
	}

	p, err := pipeline.Load(ctx, sp, emb)
	if err != nil {
		log.Fatalf("load: %v", err)
	}
	log.Printf("eval: %d papers, tier %s, %d backbone edges, %d communities",
		len(p.Papers), p.Tier(), len(p.Graph.Edges), len(p.Graph.Communities))

	results := map[string]any{
		"corpus":      len(p.Papers),
		"tier":        string(p.Tier()),
		"edges":       len(p.Graph.Edges),
		"communities": len(p.Graph.Communities),
	}
	if *all || *doRetrieve {
		results["retrieval"] = runRetrieval(p, *nA, *nB)
	}
	if *all || *doContext {
		results["context"] = runContext(p, *nC)
	}
	if *all || *doLadder {
		results["ladder"] = runLadder(p, *nB)
	}
	if *all || *doScaling {
		results["scaling"] = runScaling(*data, sp.ID, len(p.Papers))
	}
	if len(results) == 4 {
		log.Fatalf("nothing to do: pass --all or one of --retrieval --context --ladder --scaling")
	}
	if *out != "" {
		b, _ := json.MarshalIndent(results, "", "  ")
		if err := os.WriteFile(*out, b, 0o644); err != nil {
			log.Fatalf("write: %v", err)
		}
		log.Printf("eval: wrote %s", *out)
	}
}

type methodRow struct {
	Method  string  `json:"method"`
	P1      float64 `json:"p1"`
	R10     float64 `json:"r10"`
	MRR     float64 `json:"mrr"`
	NDCG10  float64 `json:"ndcg10"`
	P10     float64 `json:"p10"`
	P10Hi   float64 `json:"p10_grade2"`
	R20     float64 `json:"r20"`
	MsPerQ  float64 `json:"msPerQuery"`
	VsTFIDF eval.CI `json:"vsTfidfNdcg"`
}

// runRetrieval reproduces Table 1: known-item search and topical search with
// pooled graded judgments.
func runRetrieval(p *pipeline.Project, nA, nB int) []methodRow {
	methods := []retrieve.Method{retrieve.TFIDF, retrieve.BM25, retrieve.Dense, retrieve.Hybrid, retrieve.HybridGraph}
	if !p.Corpus.Dense.Ready() {
		methods = []retrieve.Method{retrieve.TFIDF, retrieve.BM25, retrieve.Hybrid, retrieve.HybridGraph}
	}
	eng := p.Engine

	// Task A: known-item.
	qa := eval.TaskA(p.Papers, nA, 1)
	log.Printf("eval: Task A — known-item retrieval, %d queries", len(qa))
	type aScore struct{ p1, r10, mrr []float64 }
	aScores := map[retrieve.Method]*aScore{}
	latency := map[retrieve.Method]float64{}
	for _, m := range methods {
		s := &aScore{}
		start := time.Now()
		for _, q := range qa {
			r := ranked(eng.Search(m, q.Text, 20))
			s.p1 = append(s.p1, eval.PrecisionAt(r, q.Judge, 1, 1))
			s.r10 = append(s.r10, eval.RecallAt(r, q.Judge, 10, 1))
			s.mrr = append(s.mrr, eval.ReciprocalRank(r, q.Judge))
		}
		latency[m] = float64(time.Since(start).Microseconds()) / float64(len(qa)) / 1000
		aScores[m] = s
	}

	// Task B: topical, with POOLED judgments. Every system's top 50 is judged
	// before any system is scored, so the ideal ranking is drawn from the union
	// rather than from any one system's own output.
	qb := eval.TaskB(p.Papers, nB, 2)
	log.Printf("eval: Task B — topical retrieval, %d queries, pooling top 50 across %d systems", len(qb), len(methods))
	runs := make(map[retrieve.Method][]eval.Ranked, len(methods))
	for _, m := range methods {
		rs := make([]eval.Ranked, len(qb))
		for i, q := range qb {
			rs[i] = ranked(eng.Search(m, q.Text, 50))
		}
		runs[m] = rs
	}
	for i := range qb {
		pooled := make([]eval.Ranked, 0, len(methods))
		for _, m := range methods {
			pooled = append(pooled, runs[m][i])
		}
		eval.Pool(&qb[i], p.Papers, pooled, 50)
	}

	type bScore struct{ ndcg, p10, p10hi, r20 []float64 }
	bScores := map[retrieve.Method]*bScore{}
	for _, m := range methods {
		s := &bScore{}
		for i, q := range qb {
			r := runs[m][i]
			s.ndcg = append(s.ndcg, eval.NDCGAt(r, q.Judge, 10))
			s.p10 = append(s.p10, eval.PrecisionAt(r, q.Judge, 10, 1))
			s.p10hi = append(s.p10hi, eval.PrecisionAt(r, q.Judge, 10, 2))
			s.r20 = append(s.r20, eval.RecallAt(r, q.Judge, 20, 1))
		}
		bScores[m] = s
	}

	base := bScores[retrieve.TFIDF].ndcg
	rows := make([]methodRow, 0, len(methods))
	for _, m := range methods {
		a, b := aScores[m], bScores[m]
		rows = append(rows, methodRow{
			Method: string(m),
			P1:     eval.Mean(a.p1), R10: eval.Mean(a.r10), MRR: eval.Mean(a.mrr),
			NDCG10: eval.Mean(b.ndcg), P10: eval.Mean(b.p10),
			P10Hi: eval.Mean(b.p10hi), R20: eval.Mean(b.r20),
			MsPerQ:  latency[m],
			VsTFIDF: eval.PairedBootstrap(b.ndcg, base, eval.BootstrapResamples),
		})
	}

	fmt.Println("\nTable 1 — Retrieval quality")
	fmt.Printf("%-14s %-23s  %-31s %8s\n", "", "Task A: known-item", "Task B: topical", "")
	fmt.Printf("%-14s %6s %6s %6s  %7s %6s %8s %6s %8s  %s\n",
		"Method", "P@1", "R@10", "MRR", "nDCG@10", "P@10", "P@10>=2", "R@20", "ms/q", "nDCG vs TF-IDF (95% CI)")
	for _, r := range rows {
		ci := ""
		if r.Method != string(retrieve.TFIDF) {
			mark := " "
			if r.VsTFIDF.Significant {
				mark = "*"
			}
			ci = fmt.Sprintf("%+.3f%s [%+.3f, %+.3f]", r.VsTFIDF.Diff, mark, r.VsTFIDF.Low, r.VsTFIDF.High)
		}
		fmt.Printf("%-14s %6.3f %6.3f %6.3f  %7.3f %6.3f %8.3f %6.3f %8.1f  %s\n",
			r.Method, r.P1, r.R10, r.MRR, r.NDCG10, r.P10, r.P10Hi, r.R20, r.MsPerQ, ci)
	}
	fmt.Println("* the paired-bootstrap 95% CI against TF-IDF excludes zero")
	if p.Corpus.Dense.Ready() {
		fmt.Println("Note: dense latency here is dominated by one HTTP round trip per query to encode it,")
		fmt.Println("not by the vector scan. An in-process encoder would move these rows well under a millisecond.")
	}
	return rows
}

type contextRow struct {
	Config       string  `json:"config"`
	Coverage     float64 `json:"goldCoverage"`
	Communities  float64 `json:"communitiesPerContext"`
	Docs         float64 `json:"docsPerContext"`
	VsFlatHybrid eval.CI `json:"vsFlatHybrid"`
}

// runContext reproduces Task C, the negative result: does expanding along the
// backbone improve the assembled context at a fixed budget?
func runContext(p *pipeline.Project, n int) []contextRow {
	log.Printf("eval: Task C — building the character-4-gram judge space")
	// A space no retriever uses and that played no part in building the graph.
	charTF := index.BuildCharTFIDF(p.Corpus.Abstracts, 4, 3)

	seeds := make([]int, 0, n)
	golds := make(map[int][]int, n)
	for i := range p.Papers {
		if len(seeds) >= n {
			break
		}
		g := eval.GoldSet(charTF, p.Papers, i, 5)
		if len(g) < 3 {
			continue // too few independently-judged targets to score against
		}
		seeds = append(seeds, i)
		golds[i] = g
	}
	log.Printf("eval: Task C — %d seeds with a gold set of >=3 documents", len(seeds))
	if len(seeds) == 0 {
		return nil
	}

	titles := make([]string, len(p.Papers))
	for i := range p.Papers {
		titles[i] = p.Papers[i].Title
	}
	configs := []struct {
		name string
		hops int
		eng  *retrieve.Engine
	}{
		{"flat tfidf", 0, &retrieve.Engine{Inv: p.Corpus.Inv, TF: p.Corpus.TF}},
		{"flat hybrid", 0, p.Engine},
		{"hybrid + 1 hop", 1, p.Engine},
		{"hybrid + 2 hops", 2, p.Engine},
	}
	if p.Corpus.Dense.Ready() {
		configs = append(configs, struct {
			name string
			hops int
			eng  *retrieve.Engine
		}{"flat dense", 0, &retrieve.Engine{Dense: p.Corpus.Dense, TF: p.Corpus.TF}})
	}

	per := map[string][]float64{}
	rows := make([]contextRow, 0, len(configs))
	for _, c := range configs {
		var cov, comms, docs []float64
		for _, s := range seeds {
			set := rag.AssembleContext(c.eng, titles, p.Corpus.Abstracts,
				p.Papers[s].Abstract, rag.DefaultBudgetWords, c.hops)
			got := make(map[int]bool, len(set))
			seen := map[int32]bool{}
			for _, d := range set {
				got[d.Pos] = true
				if d.Pos < len(p.Graph.Community) {
					seen[p.Graph.Community[d.Pos]] = true
				}
			}
			hit := 0
			for _, g := range golds[s] {
				if got[g] {
					hit++
				}
			}
			cov = append(cov, float64(hit)/float64(len(golds[s])))
			comms = append(comms, float64(len(seen)))
			docs = append(docs, float64(len(set)))
		}
		per[c.name] = cov
		rows = append(rows, contextRow{
			Config: c.name, Coverage: eval.Mean(cov),
			Communities: eval.Mean(comms), Docs: eval.Mean(docs),
		})
	}
	base := per["flat hybrid"]
	for i := range rows {
		rows[i].VsFlatHybrid = eval.PairedBootstrap(per[rows[i].Config], base, eval.BootstrapResamples)
	}

	fmt.Printf("\nTask C — context quality at a %d-word budget, %d seeds\n", rag.DefaultBudgetWords, len(seeds))
	fmt.Printf("%-16s %10s %12s %8s  %s\n", "Config", "coverage", "communities", "docs", "vs flat hybrid (95% CI)")
	for _, r := range rows {
		ci := ""
		if r.Config != "flat hybrid" {
			mark := " "
			if r.VsFlatHybrid.Significant {
				mark = "*"
			}
			ci = fmt.Sprintf("%+.3f%s [%+.3f, %+.3f]", r.VsFlatHybrid.Diff, mark, r.VsFlatHybrid.Low, r.VsFlatHybrid.High)
		}
		fmt.Printf("%-16s %10.3f %12.2f %8.1f  %s\n", r.Config, r.Coverage, r.Communities, r.Docs, ci)
	}
	return rows
}

type ladderRow struct {
	Tier   string  `json:"tier"`
	Adds   string  `json:"adds"`
	R10    float64 `json:"r10"`
	P1     float64 `json:"p1"`
	MsPerQ float64 `json:"msPerQuery"`
}

// runLadder reproduces Table 2a: each tier adds exactly one dependency, and
// every tier below remains a complete system.
func runLadder(p *pipeline.Project, n int) []ladderRow {
	q := eval.TaskA(p.Papers, n, 3)
	log.Printf("eval: capability ladder, %d known-item queries", len(q))
	rungs := []struct {
		tier retrieve.Tier
		adds string
		eng  *retrieve.Engine
	}{
		{retrieve.T0, "inverted index", &retrieve.Engine{Inv: p.Corpus.Inv}},
		{retrieve.T1, "+ TF-IDF", &retrieve.Engine{Inv: p.Corpus.Inv, TF: p.Corpus.TF}},
		{retrieve.T2, "+ k-NN graph", &retrieve.Engine{Inv: p.Corpus.Inv, TF: p.Corpus.TF, Graph: p.Graph}},
		{retrieve.T3, "+ encoder", &retrieve.Engine{Inv: p.Corpus.Inv, TF: p.Corpus.TF, Graph: p.Graph, Dense: p.Corpus.Dense}},
	}
	rows := make([]ladderRow, 0, len(rungs))
	for _, r := range rungs {
		if r.tier == retrieve.T3 && !p.Corpus.Dense.Ready() {
			continue
		}
		var p1, r10 []float64
		start := time.Now()
		for _, qq := range q {
			hits := ranked(r.eng.Search(retrieve.DefaultMethod(r.tier), qq.Text, 10))
			p1 = append(p1, eval.PrecisionAt(hits, qq.Judge, 1, 1))
			r10 = append(r10, eval.RecallAt(hits, qq.Judge, 10, 1))
		}
		rows = append(rows, ladderRow{
			Tier: string(r.tier), Adds: r.adds,
			R10: eval.Mean(r10), P1: eval.Mean(p1),
			MsPerQ: float64(time.Since(start).Microseconds()) / float64(len(q)) / 1000,
		})
	}
	fmt.Printf("\nTable 2a — Capability ladder, %d known-item queries\n", len(q))
	fmt.Printf("%-5s %-16s %7s %7s %8s\n", "Tier", "Adds", "R@10", "P@1", "ms/q")
	for _, r := range rows {
		fmt.Printf("%-5s %-16s %7.3f %7.3f %8.1f\n", r.Tier, r.Adds, r.R10, r.P1, r.MsPerQ)
	}
	if len(rows) > 1 {
		top := rows[len(rows)-1].R10
		if top > 0 {
			fmt.Printf("The bottom rung retains %.0f%% of top-tier recall with no model and no graph.\n",
				100*rows[0].R10/top)
		}
	}
	return rows
}

type scalingRow struct {
	Papers     int     `json:"papers"`
	IndexS     float64 `json:"indexSeconds"`
	KNNS       float64 `json:"knnSeconds"`
	ApproxS    float64 `json:"approxKnnSeconds"`
	LouvainS   float64 `json:"louvainSeconds"`
	TrendsS    float64 `json:"trendsSeconds"`
	TotalS     float64 `json:"totalSeconds"`
	RSSMB      float64 `json:"peakRssMb"`
	EdgeRecall float64 `json:"approxEdgeRecall"`
}

// runScaling reproduces Table 2b and the pruning comparison. Each corpus size
// runs in its own process so peak resident memory is that size's alone.
func runScaling(dataDir, projectID string, total int) map[string]any {
	sizes := []int{2000, 5000, 10000, 20000, total}
	var use []int
	for _, s := range sizes {
		if s <= total && (len(use) == 0 || s > use[len(use)-1]) {
			use = append(use, s)
		}
	}
	if len(use) == 0 || use[len(use)-1] != total {
		use = append(use, total)
	}
	// Small corpora need a denser sweep or the log-log fit has nothing to fit.
	if total < 4000 {
		use = nil
		for _, f := range []float64{0.2, 0.4, 0.6, 0.8, 1.0} {
			use = append(use, int(float64(total)*f))
		}
	}

	exe, err := os.Executable()
	if err != nil {
		log.Printf("eval: cannot locate self for subprocess scaling: %v", err)
		return nil
	}
	var rows []scalingRow
	for _, n := range use {
		if n < 100 {
			continue
		}
		cmd := exec.Command(exe, "--scaling-one", fmt.Sprint(n), "--project", projectID, "--data", dataDir)
		cmd.Stderr = os.Stderr
		out, err := cmd.Output()
		if err != nil {
			log.Printf("eval: scaling at n=%d failed: %v", n, err)
			continue
		}
		var row scalingRow
		if err := json.Unmarshal(out, &row); err != nil {
			log.Printf("eval: scaling at n=%d: %v", n, err)
			continue
		}
		rows = append(rows, row)
		rss := "n/a"
		if row.RSSMB > 0 {
			rss = fmt.Sprintf("%.0f MB", row.RSSMB)
		}
		log.Printf("eval: n=%d total %.2fs, RSS %s", row.Papers, row.TotalS, rss)
	}
	if len(rows) < 2 {
		return map[string]any{"rows": rows}
	}

	xs := make([]float64, len(rows))
	col := func(f func(scalingRow) float64) []float64 {
		v := make([]float64, len(rows))
		for i, r := range rows {
			v[i] = f(r)
		}
		return v
	}
	for i, r := range rows {
		xs[i] = float64(r.Papers)
	}
	exps := map[string]float64{
		"index":     eval.LogLogSlope(xs, col(func(r scalingRow) float64 { return r.IndexS })),
		"knn":       eval.LogLogSlope(xs, col(func(r scalingRow) float64 { return r.KNNS })),
		"knnApprox": eval.LogLogSlope(xs, col(func(r scalingRow) float64 { return r.ApproxS })),
		"louvain":   eval.LogLogSlope(xs, col(func(r scalingRow) float64 { return r.LouvainS })),
		"trends":    eval.LogLogSlope(xs, col(func(r scalingRow) float64 { return r.TrendsS })),
		"total":     eval.LogLogSlope(xs, col(func(r scalingRow) float64 { return r.TotalS })),
		"rss":       eval.LogLogSlope(xs, col(func(r scalingRow) float64 { return r.RSSMB })),
	}

	fmt.Println("\nTable 2b — End-to-end scaling, one process per corpus size")
	fmt.Printf("%8s %8s %8s %10s %8s %8s %8s %8s %10s\n",
		"Papers", "Index", "k-NN", "k-NN~", "Louv.", "Trends", "Total", "RSS", "edge rec.")
	fmt.Printf("%8s %8s %8s %10s %8s %8s %8s %8s %10s\n",
		"", "(s)", "(s)", "pruned(s)", "(s)", "(s)", "(s)", "(MB)", "vs exact")
	for _, r := range rows {
		fmt.Printf("%8d %8.2f %8.2f %10.2f %8.2f %8.2f %8.2f %8.0f %10.3f\n",
			r.Papers, r.IndexS, r.KNNS, r.ApproxS, r.LouvainS, r.TrendsS, r.TotalS, r.RSSMB, r.EdgeRecall)
	}
	fmt.Printf("%8s n^%.2f   n^%.2f    n^%.2f  n^%.2f  n^%.2f  n^%.2f  n^%.2f\n", "Exp.",
		exps["index"], exps["knn"], exps["knnApprox"], exps["louvain"], exps["trends"], exps["total"], exps["rss"])
	// Report what the numbers actually support. The guard used to read the
	// SMALLEST corpus and so cried "too small" on a corpus that was ample.
	last := rows[len(rows)-1]
	switch {
	case last.KNNS < 0.05 && last.Papers < 20000:
		fmt.Printf("Corpus too small to measure backbone scaling: the exact k-NN runs in %.0f ms at the largest\n"+
			"size, so the n^%.2f fit is timer resolution. Harvest at least 20,000 papers.\n",
			last.KNNS*1000, exps["knn"])
	case exps["knnApprox"] < exps["knn"] && last.ApproxS < last.KNNS:
		fmt.Printf("The exact backbone is the bottleneck at n^%.2f; inverted-index pruning brings it to n^%.2f\n"+
			"and is %.2fx faster at %d papers.\n",
			exps["knn"], exps["knnApprox"], last.KNNS/last.ApproxS, last.Papers)
	case last.ApproxS >= last.KNNS:
		fmt.Printf("The exact backbone scales as n^%.2f and costs %.2f s at %d papers. Pruning scales better\n"+
			"(n^%.2f) but is %.1fx SLOWER in wall-clock here, so it is not worth taking: this exact k-NN\n"+
			"accumulates cosine over the inverted index rather than scoring all pairs, so it never touches\n"+
			"documents sharing no term. The paper's quadratic backbone assumes a brute-force formulation.\n",
			exps["knn"], last.KNNS, last.Papers, exps["knnApprox"], last.ApproxS/last.KNNS)
	default:
		fmt.Printf("Exact backbone n^%.2f, pruned n^%.2f at %d papers.\n", exps["knn"], exps["knnApprox"], last.Papers)
	}
	fmt.Printf("Approximate-backbone fidelity: edge recall against the exact graph falls from %.3f at %d papers\n"+
		"to %.3f at %d.\n", rows[0].EdgeRecall, rows[0].Papers, last.EdgeRecall, last.Papers)

	return map[string]any{"rows": rows, "exponents": exps}
}

// runScalingOne builds one corpus size and reports its timings and peak RSS as
// JSON on stdout. It runs as a subprocess so the memory figure belongs to this
// size alone.
func runScalingOne(sp *store.Project, n int) {
	papers, err := sp.ReadPapers()
	if err != nil {
		log.Fatalf("read: %v", err)
	}
	if n > len(papers) {
		n = len(papers)
	}
	papers = papers[:n]

	total := time.Now()
	t := time.Now()
	c := pipeline.BuildLexical(papers, pipeline.DefaultBuildOptions())
	row := scalingRow{Papers: n, IndexS: time.Since(t).Seconds()}

	t = time.Now()
	exact := graph.BuildKNN(c.TF, graph.DefaultKNNOptions())
	row.KNNS = time.Since(t).Seconds()

	opt := graph.DefaultKNNOptions()
	opt.Approximate = true
	t = time.Now()
	approx := graph.BuildKNN(c.TF, opt)
	row.ApproxS = time.Since(t).Seconds()

	exactSet := make(map[[2]int32]struct{}, len(exact))
	for _, e := range exact {
		exactSet[[2]int32{e.A, e.B}] = struct{}{}
	}
	hit := 0
	for _, e := range approx {
		if _, ok := exactSet[[2]int32{e.A, e.B}]; ok {
			hit++
		}
	}
	if len(exact) > 0 {
		row.EdgeRecall = float64(hit) / float64(len(exact))
	}

	t = time.Now()
	g := graph.Build(c.TF, papers, graph.DefaultKNNOptions())
	// Build re-runs the k-NN, so Louvain's own cost is what is left over.
	row.LouvainS = math.Max(time.Since(t).Seconds()-row.KNNS, 0)

	t = time.Now()
	an := analyze.BuildAnalytics(papers, g.Community, len(g.Communities), 2)
	_ = analyze.Gaps(g, analyze.DefaultPermutations)
	row.TrendsS = time.Since(t).Seconds()
	runtime.KeepAlive(an)

	row.TotalS = time.Since(total).Seconds()
	// A zero RSS means "not measured on this platform", which LogLogSlope
	// already skips rather than fitting through.
	row.RSSMB, _ = peakRSSMB()
	runtime.KeepAlive(g)

	b, _ := json.Marshal(row)
	os.Stdout.Write(b)
}

func ranked(hits []index.Hit) eval.Ranked {
	out := make(eval.Ranked, len(hits))
	for i, h := range hits {
		out[i] = h.Doc
	}
	return out
}
