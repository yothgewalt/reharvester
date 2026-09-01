package graph

import (
	"math/rand/v2"
	"sort"

	"gonum.org/v1/gonum/graph/community"
	"gonum.org/v1/gonum/graph/network"
	"gonum.org/v1/gonum/graph/simple"

	"github.com/yothgewalt/reharvester/internal/index"
	"github.com/yothgewalt/reharvester/internal/paper"
)

// Community is one Louvain partition, labelled by the highest-weight terms of
// its TF-IDF centroid.
type Community struct {
	ID       int               `json:"id"`
	Size     int               `json:"size"`
	Label    string            `json:"label"`
	Terms    []string          `json:"terms"`
	Members  []int32           `json:"members"`
	Centroid map[int32]float32 `json:"-"`
}

// Graph is the backbone plus everything derived from it. The backbone, not the
// raw index, is what makes the system navigable.
type Graph struct {
	N           int         `json:"n"`
	Edges       []Edge      `json:"edges"`
	Coauthor    []Edge      `json:"coauthor"`
	Community   []int32     `json:"community"`
	Communities []Community `json:"communities"`
	PageRank    []float64   `json:"pageRank"`
	Bridge      []float64   `json:"bridge"`

	adj  [][]int32
	adjW [][]float32
}

// Neighbors satisfies retrieve.Neighbours, so the retrieval engine can expand
// along the backbone without depending on this package's concrete type.
func (g *Graph) Neighbors(doc int) []int {
	if g == nil || doc < 0 || doc >= len(g.adj) {
		return nil
	}
	out := make([]int, len(g.adj[doc]))
	for i, v := range g.adj[doc] {
		out[i] = int(v)
	}
	return out
}

// NeighborsWeighted returns the same neighbours as Neighbors plus the cosine on
// each backbone edge, in matching order. Callers wanting similarity order must
// sort; the adjacency is in edge-emission order.
func (g *Graph) NeighborsWeighted(doc int) ([]int, []float32) {
	if g == nil || doc < 0 || doc >= len(g.adj) {
		return nil, nil
	}
	return g.Neighbors(doc), g.adjW[doc]
}

func (g *Graph) Degree(doc int) int {
	if g == nil || doc < 0 || doc >= len(g.adj) {
		return 0
	}
	return len(g.adj[doc])
}

// Index rebuilds the adjacency lists. Call after loading a Graph from disk,
// where the unexported adjacency does not survive the round trip.
func (g *Graph) Index() {
	g.adj = make([][]int32, g.N)
	g.adjW = make([][]float32, g.N)
	for _, e := range g.Edges {
		g.adj[e.A] = append(g.adj[e.A], e.B)
		g.adj[e.B] = append(g.adj[e.B], e.A)
		g.adjW[e.A] = append(g.adjW[e.A], e.W)
		g.adjW[e.B] = append(g.adjW[e.B], e.W)
	}
}

// MaxAuthorPapers excludes prolific author strings from the co-authorship
// layer. Beyond this many papers a name is far more likely to be a collision
// between distinct people than a real collaboration hub.
const MaxAuthorPapers = 60

// BuildCoauthor links papers sharing an author, skipping names that appear on
// more than MaxAuthorPapers papers.
func BuildCoauthor(papers []paper.Paper) []Edge {
	byAuthor := make(map[string][]int32)
	for i, p := range papers {
		for _, a := range p.Authors {
			byAuthor[a] = append(byAuthor[a], int32(i))
		}
	}
	seen := make(map[[2]int32]struct{})
	var edges []Edge
	for _, docs := range byAuthor {
		if len(docs) < 2 || len(docs) > MaxAuthorPapers {
			continue
		}
		for i := range docs {
			for j := i + 1; j < len(docs); j++ {
				a, b := docs[i], docs[j]
				if a > b {
					a, b = b, a
				}
				k := [2]int32{a, b}
				if _, dup := seen[k]; dup {
					continue
				}
				seen[k] = struct{}{}
				edges = append(edges, Edge{A: a, B: b, W: 1})
			}
		}
	}
	sort.Slice(edges, func(a, b int) bool {
		if edges[a].A != edges[b].A {
			return edges[a].A < edges[b].A
		}
		return edges[a].B < edges[b].B
	})
	return edges
}

// Build assembles the full graph: the mutual k-NN backbone, the co-authorship
// layer, Louvain communities over the backbone, PageRank, and the per-node
// bridge score.
func Build(tf *index.TFIDF, papers []paper.Paper, opt KNNOptions) *Graph {
	g := &Graph{
		N:        tf.N,
		Edges:    BuildKNN(tf, opt),
		Coauthor: BuildCoauthor(papers),
	}
	g.Index()
	g.detectCommunities(tf)
	g.pageRank()
	g.bridgeScores()
	return g
}

// MinCommunitySize is the floor for a Louvain partition to be treated as a
// community at all, as a function of corpus size.
//
// A sparse mutual k-NN backbone leaves isolated and near-isolated documents, and
// Louvain hands each one its own partition: on 24,000 papers that produced 1,071
// "communities", 899 of them singletons. Keeping those wrecks everything
// downstream — the specificity filter normalises by log|C| and so treats almost
// every term as topical, the gap analysis explodes to half a million pairs of
// mostly-singletons, and the graph snapshot mints six concept nodes per
// partition. Below this floor a document is simply unclustered.
func MinCommunitySize(n int) int {
	if m := n / 500; m > 10 {
		return m
	}
	return 10
}

// detectCommunities partitions the cosine-weighted backbone with Louvain, keeps
// the partitions large enough to support a centroid, and labels each by its
// heaviest terms.
//
// Kept communities are renumbered densely, so Community[i] indexes directly into
// Communities and -1 means unclustered. Everything downstream relies on that.
func (g *Graph) detectCommunities(tf *index.TFIDF) {
	g.Community = make([]int32, g.N)
	for i := range g.Community {
		g.Community[i] = -1
	}
	if len(g.Edges) == 0 {
		return
	}
	wg := simple.NewWeightedUndirectedGraph(0, 0)
	for i := range g.N {
		wg.AddNode(simple.Node(i))
	}
	for _, e := range g.Edges {
		wg.SetWeightedEdge(simple.WeightedEdge{
			F: simple.Node(e.A), T: simple.Node(e.B), W: float64(e.W),
		})
	}
	// A fixed seed keeps a rebuild of the same corpus reproducible, which is
	// the whole point of a search a researcher can repeat and cite.
	reduced := community.Modularize(wg, 1.0, rand.New(rand.NewPCG(1, 2)))
	parts := reduced.Communities()

	min := MinCommunitySize(g.N)
	kept := make([][]int32, 0, len(parts))
	for _, members := range parts {
		if len(members) < min {
			continue
		}
		ids := make([]int32, 0, len(members))
		for _, nd := range members {
			ids = append(ids, int32(nd.ID()))
		}
		sort.Slice(ids, func(a, b int) bool { return ids[a] < ids[b] })
		kept = append(kept, ids)
	}
	// Largest first, then renumber so an id is an index into Communities.
	sort.Slice(kept, func(a, b int) bool { return len(kept[a]) > len(kept[b]) })

	terms := inverseVocab(tf)
	for cid, ids := range kept {
		for _, id := range ids {
			g.Community[id] = int32(cid)
		}
		c := Community{ID: cid, Size: len(ids), Members: ids, Centroid: centroid(tf, ids)}
		c.Terms = topTerms(c.Centroid, terms, 6)
		if len(c.Terms) > 0 {
			c.Label = c.Terms[0]
			for _, t := range c.Terms[1:min3(3, len(c.Terms))] {
				c.Label += " " + t
			}
		}
		g.Communities = append(g.Communities, c)
	}
}

func min3(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// centroid is the mean TF-IDF vector of a community's members.
func centroid(tf *index.TFIDF, members []int32) map[int32]float32 {
	c := make(map[int32]float32)
	if len(members) == 0 {
		return c
	}
	for _, m := range members {
		ids, ws := tf.Row(int(m))
		for j, id := range ids {
			c[id] += ws[j]
		}
	}
	inv := float32(1) / float32(len(members))
	for id := range c {
		c[id] *= inv
	}
	return c
}

func topTerms(c map[int32]float32, terms []string, k int) []string {
	type tw struct {
		id int32
		w  float32
	}
	all := make([]tw, 0, len(c))
	for id, w := range c {
		all = append(all, tw{id, w})
	}
	sort.Slice(all, func(a, b int) bool {
		if all[a].w != all[b].w {
			return all[a].w > all[b].w
		}
		return all[a].id < all[b].id
	})
	out := make([]string, 0, k)
	for _, t := range all {
		if len(out) == k {
			break
		}
		if int(t.id) < len(terms) {
			out = append(out, terms[t.id])
		}
	}
	return out
}

func inverseVocab(tf *index.TFIDF) []string {
	out := make([]string, len(tf.IDF))
	for t, id := range tf.Vocab {
		out[id] = t
	}
	return out
}

// pageRank runs over a bidirected copy of the backbone: gonum's PageRank takes a
// directed graph and ignores edge weights, so each undirected edge becomes an
// arc in both directions.
//
// PageRankSparse, not PageRank: the dense variant materialises an n-by-n matrix,
// which at 24,000 papers is a 4.6 GB allocation and turned a sub-second stage
// into eighty-four seconds. The backbone has about four edges per node, so the
// sparse formulation is the only sane one here.
func (g *Graph) pageRank() {
	g.PageRank = make([]float64, g.N)
	if len(g.Edges) == 0 {
		return
	}
	dg := simple.NewDirectedGraph()
	for i := range g.N {
		dg.AddNode(simple.Node(i))
	}
	for _, e := range g.Edges {
		dg.SetEdge(simple.Edge{F: simple.Node(e.A), T: simple.Node(e.B)})
		dg.SetEdge(simple.Edge{F: simple.Node(e.B), T: simple.Node(e.A)})
	}
	for id, pr := range network.PageRankSparse(dg, 0.85, 1e-6) {
		if id >= 0 && int(id) < g.N {
			g.PageRank[id] = pr
		}
	}
}

// bridgeScores records, per node, the fraction of its backbone edge weight that
// crosses a community boundary. This operationalises the structural-hole
// intuition at document level: a paper sitting between topics scores high.
func (g *Graph) bridgeScores() {
	g.Bridge = make([]float64, g.N)
	total := make([]float64, g.N)
	cross := make([]float64, g.N)
	for _, e := range g.Edges {
		w := float64(e.W)
		total[e.A] += w
		total[e.B] += w
		if g.Community[e.A] != g.Community[e.B] {
			cross[e.A] += w
			cross[e.B] += w
		}
	}
	for i := range g.N {
		if total[i] > 0 {
			g.Bridge[i] = cross[i] / total[i]
		}
	}
}
