package httpapi

import (
	"fmt"
	"sort"
	"time"

	"github.com/yothgewalt/reharvester/internal/analyze"
	"github.com/yothgewalt/reharvester/internal/paper"
	"github.com/yothgewalt/reharvester/internal/pipeline"
)

// DefaultSnapshotNodes caps how many papers a snapshot carries.
//
// This snapshot backs the reader, not the field map: Fig. 2c is drawn from
// /api/v1/communities and its link counts, which aggregate the whole corpus and
// so are unaffected by this cut.
//
// Edges grow far faster than nodes here — both endpoints must be inside the cut
// — so payload is the binding constraint. Measured on the 24k paper corpus:
// 1500 nodes is 2.1 MB, 4000 is 6.2 MB, 6000 is 11 MB, and the whole corpus is
// 83 MB.
//
// Anything outside this cut has no node id, so the reader renders it as inert
// text: /ask sources and community members are both subsets of these nodes.
const DefaultSnapshotNodes = 4000

// ConceptsPerCommunity is how many centroid terms become concept nodes. The
// reader linkifies these labels inside abstracts, so they are what makes a
// [[wikilink]] resolvable; without them an abstract is plain prose.
const ConceptsPerCommunity = 6

// conceptInfo is a concept node together with the papers it was drawn from.
type conceptInfo struct {
	id      string
	label   string
	cluster string
	docs    []int
}

// Snapshots holds the two graph views plus the gap node ids derived from the
// same node set — the UI's gap overlay matches ids against the snapshot it is
// already showing, so they must be computed together.
type Snapshots struct {
	Knowledge   GraphSnapshot
	Cooccurring GraphSnapshot
	GapNodeIDs  []string
	DocIDByPos  map[int]string
	PosByDocID  map[string]int
	ConceptDocs map[string][]int  // concept node id -> member paper positions
	ConceptOf   map[string]string // concept node id -> its community label
	Labels      map[string]string
}

// BuildSnapshots renders a project into the two views the UI asks for.
func BuildSnapshots(p *pipeline.Project, limit int) *Snapshots {
	if limit <= 0 {
		limit = DefaultSnapshotNodes
	}
	g := p.Graph
	n := len(p.Papers)

	// Rank papers by PageRank so the coarse-grained map keeps the structurally
	// important documents rather than an arbitrary prefix.
	order := make([]int, n)
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool {
		if g != nil && len(g.PageRank) == n {
			if g.PageRank[order[a]] != g.PageRank[order[b]] {
				return g.PageRank[order[a]] > g.PageRank[order[b]]
			}
		}
		return order[a] < order[b]
	})
	if len(order) > limit {
		order = order[:limit]
	}
	included := make(map[int]bool, len(order))
	for _, i := range order {
		included[i] = true
	}

	s := &Snapshots{
		DocIDByPos:  make(map[int]string, len(order)),
		PosByDocID:  make(map[string]int, len(order)),
		ConceptDocs: make(map[string][]int),
		ConceptOf:   make(map[string]string),
		Labels:      make(map[string]string),
	}
	// Node ids must be unique: two papers can share a title, and a concept term
	// can slug to the same string as a paper.
	taken := make(map[string]bool)
	unique := func(base string) string {
		if base == "" {
			base = "untitled"
		}
		id := base
		for k := 2; taken[id]; k++ {
			id = fmt.Sprintf("%s-%d", base, k)
		}
		taken[id] = true
		return id
	}

	clusterOf := func(pos int) string {
		if g == nil || pos >= len(g.Community) {
			return "unclustered"
		}
		cid := g.Community[pos]
		if cid < 0 || int(cid) >= len(g.Communities) {
			return "unclustered"
		}
		return paper.Slug(g.Communities[cid].Label)
	}

	gapSet := make(map[int]bool)
	if g != nil {
		for _, id := range analyze.GapNodes(g, p.Gaps, 8, limit/10+1) {
			gapSet[int(id)] = true
		}
	}

	nodes := make([]GraphNode, 0, len(order)+len(g.Communities)*ConceptsPerCommunity)
	for _, pos := range order {
		pp := p.Papers[pos]
		id := unique(paper.Slug(pp.Title))
		s.DocIDByPos[pos] = id
		s.PosByDocID[id] = pos
		s.Labels[id] = pp.Title
		pr := 0.0
		if g != nil && pos < len(g.PageRank) {
			pr = g.PageRank[pos]
		}
		bridge := 0.0
		if g != nil && pos < len(g.Bridge) {
			bridge = g.Bridge[pos]
		}
		category := ""
		if len(pp.Categories) > 0 {
			category = pp.Categories[0]
		}
		nodes = append(nodes, GraphNode{
			ID: id, Label: pp.Title,
			Data: GraphNodeData{
				Kind: "paper", DocID: id, Year: pp.Year(), PageRank: pr,
				Gap: gapSet[pos], Bridge: bridge, Cluster: clusterOf(pos),
				Category: category, OpenAccess: pp.OpenAccess,
			},
		})
	}

	// Concept nodes: the centroid terms that label each community.
	var concepts []conceptInfo
	if g != nil {
		for _, c := range g.Communities {
			cluster := paper.Slug(c.Label)
			for i, term := range c.Terms {
				if i >= ConceptsPerCommunity {
					break
				}
				id := unique(paper.Slug(term))
				members := make([]int, 0, 8)
				for _, m := range c.Members {
					if included[int(m)] {
						members = append(members, int(m))
					}
					if len(members) >= 12 {
						break
					}
				}
				if len(members) == 0 {
					continue
				}
				s.Labels[id] = term
				s.ConceptDocs[id] = members
				s.ConceptOf[id] = c.Label
				concepts = append(concepts, conceptInfo{id: id, label: term, cluster: cluster, docs: members})
				nodes = append(nodes, GraphNode{
					ID: id, Label: term,
					Data: GraphNodeData{
						Kind: "concept", DocID: id, Year: 0, PageRank: 0,
						Gap: false, Cluster: cluster, OpenAccess: true,
					},
				})
			}
		}
	}

	edges := make([]GraphEdge, 0, len(order)*4)
	seenEdge := make(map[string]bool)
	addEdge := func(a, b string, w float64, rel string) {
		if a == b || a == "" || b == "" {
			return
		}
		id := a + "->" + b
		if seenEdge[id] {
			return
		}
		seenEdge[id] = true
		edges = append(edges, GraphEdge{ID: id, Source: a, Target: b, Weight: w, Relation: rel})
	}
	if g != nil {
		for _, e := range g.Edges {
			if included[int(e.A)] && included[int(e.B)] {
				addEdge(s.DocIDByPos[int(e.A)], s.DocIDByPos[int(e.B)], float64(e.W), "similar-to")
			}
		}
		for _, e := range g.Coauthor {
			if included[int(e.A)] && included[int(e.B)] {
				addEdge(s.DocIDByPos[int(e.A)], s.DocIDByPos[int(e.B)], float64(e.W), "co-authored-with")
			}
		}
	}
	for _, c := range concepts {
		for _, d := range c.docs {
			addEdge(c.id, s.DocIDByPos[d], 1, "appears-in")
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	s.Knowledge = GraphSnapshot{Nodes: nodes, Edges: edges, GeneratedAt: now}
	s.Cooccurring = buildCooccurrence(concepts, now)

	for _, nd := range nodes {
		if nd.Data.Gap {
			s.GapNodeIDs = append(s.GapNodeIDs, nd.ID)
		}
	}
	return s
}

// buildCooccurrence links concepts that share papers, weighting by how many.
// The UI sizes these nodes by Frequency and draws no arrowheads.
func buildCooccurrence(concepts []conceptInfo, now string) GraphSnapshot {
	freq := make(map[string]int, len(concepts))
	var edges []GraphEdge
	for i := range concepts {
		for j := i + 1; j < len(concepts); j++ {
			shared := overlap(concepts[i].docs, concepts[j].docs)
			if shared == 0 {
				continue
			}
			a, b := concepts[i].id, concepts[j].id
			edges = append(edges, GraphEdge{
				ID: a + "->" + b, Source: a, Target: b,
				Weight: float64(shared), Relation: "co-occurs-with",
			})
			freq[a] += shared
			freq[b] += shared
		}
	}
	nodes := make([]GraphNode, 0, len(concepts))
	for _, c := range concepts {
		f := freq[c.id]
		nodes = append(nodes, GraphNode{
			ID: c.id, Label: c.label,
			Data: GraphNodeData{
				Kind: "concept", DocID: c.id, Cluster: c.cluster,
				OpenAccess: true, Frequency: &f,
			},
		})
	}
	return GraphSnapshot{Nodes: nodes, Edges: edges, GeneratedAt: now}
}

func overlap(a, b []int) int {
	set := make(map[int]struct{}, len(a))
	for _, x := range a {
		set[x] = struct{}{}
	}
	n := 0
	for _, y := range b {
		if _, ok := set[y]; ok {
			n++
		}
	}
	return n
}
