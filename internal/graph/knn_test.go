package graph

import (
	"fmt"
	"strings"
	"testing"

	"github.com/yothgewalt/reharvester/internal/index"
)

// abstracts builds n documents over four topics whose vocabularies overlap, so
// top-k lists are asymmetric and the mutual filter actually drops arcs.
func abstracts(n int) []string {
	vocab := []string{"orbit", "galaxy", "halo", "lensing", "neutrino", "detector", "collider", "quark", "lattice", "spin"}
	docs := make([]string, n)
	for i := range n {
		var b strings.Builder
		topic := i % 4
		for j := range 5 {
			w := vocab[(topic*2+j)%len(vocab)]
			fmt.Fprintf(&b, "%s %s%d ", w, w, (i+j)%3)
		}
		docs[i] = b.String()
	}
	return docs
}

func TestNearestArcsAreTheTopKListsTheBackboneFiltersFrom(t *testing.T) {
	tf := index.BuildTFIDF(abstracts(40), 1, 1)
	for _, approx := range []bool{false, true} {
		opt := DefaultKNNOptions()
		opt.Approximate = approx
		top := nearestLists(tf, opt)
		arcs := nearestArcs(top)

		out := map[int32]int{}
		has := map[[2]int32]float32{}
		for _, a := range arcs {
			if a.A == a.B {
				t.Fatalf("approx=%v: self arc on %d", approx, a.A)
			}
			if a.W <= 0 {
				t.Fatalf("approx=%v: arc %d->%d has weight %v", approx, a.A, a.B, a.W)
			}
			out[a.A]++
			has[[2]int32{a.A, a.B}] = a.W
		}
		for doc, k := range out {
			if k > opt.K {
				t.Errorf("approx=%v: doc %d has %d arcs, want <= %d", approx, doc, k, opt.K)
			}
		}

		mutual := map[[2]int32]float32{}
		for pair, w := range has {
			if pair[0] < pair[1] {
				if _, back := has[[2]int32{pair[1], pair[0]}]; back {
					mutual[pair] = w
				}
			}
		}
		edges := mutualEdges(top, opt.K)
		if len(edges) != len(mutual) {
			t.Fatalf("approx=%v: %d mutual edges, but %d pairs have arcs both ways", approx, len(edges), len(mutual))
		}
		for _, e := range edges {
			if w, ok := mutual[[2]int32{e.A, e.B}]; !ok || w != e.W {
				t.Errorf("approx=%v: edge %d-%d (w=%v) not matched by arcs (w=%v, ok=%v)", approx, e.A, e.B, e.W, w, ok)
			}
		}
		if len(arcs) == 2*len(edges) {
			t.Errorf("approx=%v: every arc is mutual; the fixture no longer exercises one-way arcs", approx)
		}
	}
}

func TestBuildKeepsTheBackboneBuildKNNProduces(t *testing.T) {
	tf := index.BuildTFIDF(abstracts(40), 1, 1)
	g := Build(tf, nil, DefaultKNNOptions())
	want := BuildKNN(tf, DefaultKNNOptions())
	if len(g.Edges) != len(want) {
		t.Fatalf("Build has %d edges, BuildKNN %d", len(g.Edges), len(want))
	}
	for i := range want {
		if g.Edges[i] != want[i] {
			t.Fatalf("edge %d: Build %v, BuildKNN %v", i, g.Edges[i], want[i])
		}
	}
	if g.Nearest == nil {
		t.Error("Build left Nearest nil; Load would rebuild it forever")
	}
}
